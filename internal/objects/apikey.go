package objects

import (
	"slices"
)

type APIKeyProfiles struct {
	ActiveProfile string          `json:"activeProfile"`
	Profiles      []APIKeyProfile `json:"profiles"`
}

type APIKeyProfile struct {
	Name string `json:"name"`
	// TemplateID links this profile to the template it was loaded from. A nil
	// value means the profile is independently managed. The pointer keeps old
	// serialized profiles fully backward compatible.
	TemplateID          *int           `json:"templateID,omitempty"`
	TemplateName        string         `json:"templateName,omitempty"`
	ModelMappings       []ModelMapping `json:"modelMappings"`
	LoadBalanceStrategy *string        `json:"loadBalanceStrategy,omitempty"`
	TraceStickyMode     *string        `json:"traceStickyMode,omitempty"`

	ChannelIDs           []int                `json:"channelIDs,omitempty"`
	ChannelTags          []string             `json:"channelTags,omitempty"`
	ChannelTagsMatchMode ChannelTagsMatchMode `json:"channelTagsMatchMode,omitempty"`
	ModelIDs             []string             `json:"modelIDs,omitempty"`
}

// ChannelTagsMatchMode controls how profile channel tags are matched.
// If this enum is changed, update MatchChannelTags in this file.
type ChannelTagsMatchMode string

const (
	ChannelTagsMatchModeAny  ChannelTagsMatchMode = "any"
	ChannelTagsMatchModeAll  ChannelTagsMatchMode = "all"
	ChannelTagsMatchModeNone ChannelTagsMatchMode = "none"
)

func (m ChannelTagsMatchMode) IsValid() bool {
	return m == "" || m == ChannelTagsMatchModeAny || m == ChannelTagsMatchModeAll || m == ChannelTagsMatchModeNone
}

func (m ChannelTagsMatchMode) OrDefault() ChannelTagsMatchMode {
	if m == ChannelTagsMatchModeAll {
		return ChannelTagsMatchModeAll
	}

	if m == ChannelTagsMatchModeNone {
		return ChannelTagsMatchModeNone
	}

	return ChannelTagsMatchModeAny
}

func (p *APIKeyProfile) MatchChannelTags(tags []string) bool {
	if p == nil || len(p.ChannelTags) == 0 {
		return true
	}

	return MatchChannelTags(p.ChannelTags, p.ChannelTagsMatchMode, tags)
}

func MatchChannelTags(allowedTags []string, matchMode ChannelTagsMatchMode, channelTags []string) bool {
	//nolint:exhaustive // Checked.
	switch matchMode.OrDefault() {
	case ChannelTagsMatchModeAll:
		for _, allowedTag := range allowedTags {
			if !slices.Contains(channelTags, allowedTag) {
				return false
			}
		}

		return true
	case ChannelTagsMatchModeNone:
		for _, tag := range channelTags {
			if slices.Contains(allowedTags, tag) {
				return false
			}
		}

		return true
	default:
		for _, tag := range channelTags {
			if slices.Contains(allowedTags, tag) {
				return true
			}
		}

		return false
	}
}

func (p *APIKeyProfile) Clone() *APIKeyProfile {
	if p == nil {
		return nil
	}
	cp := *p
	if p.TemplateID != nil {
		id := *p.TemplateID
		cp.TemplateID = &id
	}
	if len(p.ModelMappings) > 0 {
		cp.ModelMappings = make([]ModelMapping, len(p.ModelMappings))
		copy(cp.ModelMappings, p.ModelMappings)
	}
	if len(p.ChannelIDs) > 0 {
		cp.ChannelIDs = make([]int, len(p.ChannelIDs))
		copy(cp.ChannelIDs, p.ChannelIDs)
	}
	if len(p.ChannelTags) > 0 {
		cp.ChannelTags = make([]string, len(p.ChannelTags))
		copy(cp.ChannelTags, p.ChannelTags)
	}
	if len(p.ModelIDs) > 0 {
		cp.ModelIDs = make([]string, len(p.ModelIDs))
		copy(cp.ModelIDs, p.ModelIDs)
	}
	if p.LoadBalanceStrategy != nil {
		s := *p.LoadBalanceStrategy
		cp.LoadBalanceStrategy = &s
	}
	if p.TraceStickyMode != nil {
		s := *p.TraceStickyMode
		cp.TraceStickyMode = &s
	}
	return &cp
}
