package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

type PromptServiceParams struct {
	fx.In

	Ent *ent.Client
}

func NewPromptService(params PromptServiceParams) *PromptService {
	return &PromptService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

func (svc *PromptService) Initialize(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "prompt-initialize")

	if err := svc.loadPrompts(ctx); err != nil {
		log.Error(ctx, "failed to load prompts", log.Cause(err))
	}

	log.Info(ctx, "initial prompt loading completed")

	return nil
}

type PromptService struct {
	*AbstractService

	mu                     sync.Mutex
	cachedEnabledPrompts   []*ent.Prompt
	latestCachedUpdateTime time.Time
}

func (svc *PromptService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "prompt-cache",
		Description: "Refresh prompt cache every minute",
		CronExpr:    "*/1 * * * *",
		Timezone:    "UTC",
	}, svc.loadPromptsPeriodic)
}

func (svc *PromptService) loadPromptsPeriodic(ctx context.Context) {
	if err := svc.loadPrompts(ctx); err != nil {
		log.Error(ctx, "failed to reload prompts periodically", log.Cause(err))
	}
}

func (svc *PromptService) ValidatePromptSettings(settings objects.PromptSettings) error {
	if len(settings.Conditions) == 0 {
		return nil
	}

	for _, composite := range settings.Conditions {
		for _, condition := range composite.Conditions {
			if condition.Type == objects.PromptActivationConditionTypeModelPattern {
				if condition.ModelPattern == nil || *condition.ModelPattern == "" {
					return fmt.Errorf("model_pattern is required when type is model_pattern")
				}

				if err := xregexp.ValidateRegex(*condition.ModelPattern); err != nil {
					return fmt.Errorf("invalid regex pattern in model_pattern: %w", err)
				}
			}

			if condition.Type == objects.PromptActivationConditionTypeModelID {
				if condition.ModelID == nil || *condition.ModelID == "" {
					return fmt.Errorf("model_id is required when type is model_id")
				}
			}

			if condition.Type == objects.PromptActivationConditionTypeAPIKey {
				if condition.APIKeyID == nil {
					return fmt.Errorf("api_key_id is required when type is api_key")
				}

				if *condition.APIKeyID <= 0 {
					return fmt.Errorf("api_key_id must be greater than 0")
				}
			}
		}
	}

	return nil
}

func (svc *PromptService) CreatePrompt(ctx context.Context, input ent.CreatePromptInput) (*ent.Prompt, error) {
	if err := svc.ValidatePromptSettings(input.Settings); err != nil {
		return nil, err
	}

	// Check for duplicate prompt name
	exists, err := svc.entFromContext(ctx).Prompt.Query().
		Where(prompt.Name(input.Name)).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check prompt name uniqueness: %w", err)
	}

	if exists {
		return nil, xerrors.DuplicateNameError("prompt", input.Name)
	}

	createBuilder := svc.entFromContext(ctx).Prompt.Create().
		SetName(input.Name).
		SetRole(input.Role).
		SetContent(input.Content).
		SetNillableOrder(input.Order).
		SetNillableDescription(input.Description).
		SetNillableStatus(input.Status).
		SetSettings(input.Settings)

	prompt, err := createBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt: %w", err)
	}

	svc.asyncReloadPrompts()

	return prompt, nil
}

func (svc *PromptService) UpdatePrompt(ctx context.Context, id int, input *ent.UpdatePromptInput) (*ent.Prompt, error) {
	if input.Settings != nil {
		if err := svc.ValidatePromptSettings(*input.Settings); err != nil {
			return nil, err
		}
	}

	// Check for duplicate name if being updated
	if input.Name != nil {
		exists, err := svc.entFromContext(ctx).Prompt.Query().
			Where(
				prompt.Name(*input.Name),
				prompt.IDNEQ(id),
			).
			Exist(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check prompt name uniqueness: %w", err)
		}

		if exists {
			return nil, xerrors.DuplicateNameError("prompt", *input.Name)
		}
	}

	updateBuilder := svc.entFromContext(ctx).Prompt.Update().
		Where(prompt.IDEQ(id)).
		SetNillableName(input.Name).
		SetNillableDescription(input.Description).
		SetNillableRole(input.Role).
		SetNillableContent(input.Content).
		SetNillableOrder(input.Order).
		SetNillableStatus(input.Status)

	if input.Settings != nil {
		updateBuilder.SetSettings(*input.Settings)
	}

	n, err := updateBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt: %w", err)
	}

	if n == 0 {
		return nil, fmt.Errorf("prompt not found")
	}

	p, err := svc.entFromContext(ctx).Prompt.Query().
		Where(prompt.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated prompt: %w", err)
	}

	svc.asyncReloadPrompts()

	return p, nil
}

func (svc *PromptService) DeletePrompt(ctx context.Context, id int) error {
	n, err := svc.entFromContext(ctx).Prompt.Delete().
		Where(prompt.IDEQ(id)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete prompt: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("prompt not found")
	}

	svc.asyncReloadPrompts()

	return nil
}

func (svc *PromptService) UpdatePromptStatus(ctx context.Context, id int, status prompt.Status) (*ent.Prompt, error) {
	n, err := svc.entFromContext(ctx).Prompt.Update().
		Where(prompt.IDEQ(id)).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt status: %w", err)
	}

	if n == 0 {
		return nil, fmt.Errorf("prompt not found")
	}

	p, err := svc.entFromContext(ctx).Prompt.Query().
		Where(prompt.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated prompt: %w", err)
	}

	svc.asyncReloadPrompts()

	return p, nil
}

func (svc *PromptService) BulkDeletePrompts(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Prompt.Delete().
		Where(prompt.IDIn(ids...)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk delete prompts: %w", err)
	}

	svc.asyncReloadPrompts()

	return nil
}

func (svc *PromptService) BulkEnablePrompts(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Prompt.Update().
		Where(prompt.IDIn(ids...)).
		SetStatus(prompt.StatusEnabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk enable prompts: %w", err)
	}

	svc.asyncReloadPrompts()

	return nil
}

func (svc *PromptService) BulkDisablePrompts(ctx context.Context, ids []int) error {
	_, err := svc.entFromContext(ctx).Prompt.Update().
		Where(prompt.IDIn(ids...)).
		SetStatus(prompt.StatusDisabled).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk disable prompts: %w", err)
	}

	svc.asyncReloadPrompts()

	return nil
}

func (svc *PromptService) loadPrompts(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "prompt-load-cache")

	svc.mu.Lock()
	defer svc.mu.Unlock()

	// Check if there are updates
	latestUpdatedPrompt, err := svc.entFromContext(ctx).Prompt.Query().
		Order(ent.Desc(prompt.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	if latestUpdatedPrompt != nil {
		if !latestUpdatedPrompt.UpdatedAt.After(svc.latestCachedUpdateTime) {
			return nil
		}

		svc.latestCachedUpdateTime = latestUpdatedPrompt.UpdatedAt
	} else {
		svc.latestCachedUpdateTime = time.Time{}
	}

	entities, err := svc.entFromContext(ctx).Prompt.Query().
		Where(prompt.StatusEQ(prompt.StatusEnabled)).
		All(ctx)
	if err != nil {
		return err
	}

	svc.cachedEnabledPrompts = entities

	return nil
}

func (svc *PromptService) GetEnabledPrompts(ctx context.Context) ([]*ent.Prompt, error) {
	svc.mu.Lock()
	prompts := svc.cachedEnabledPrompts
	svc.mu.Unlock()

	if prompts != nil {
		return prompts, nil
	}

	if err := svc.loadPrompts(ctx); err != nil {
		return nil, err
	}

	svc.mu.Lock()
	prompts = svc.cachedEnabledPrompts
	svc.mu.Unlock()

	return prompts, nil
}

func (svc *PromptService) asyncReloadPrompts() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(context.Background(), "panic in async reload prompts", log.Any("panic", r))
			}
		}()

		reloadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Force reload by resetting latestUpdate timestamp
		svc.mu.Lock()
		svc.latestCachedUpdateTime = time.Time{}
		svc.mu.Unlock()

		if reloadErr := svc.loadPrompts(reloadCtx); reloadErr != nil {
			log.Error(reloadCtx, "failed to reload prompts", log.Cause(reloadErr))
		}
	}()
}
