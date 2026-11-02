import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { defaultSelection, readSelection, resolveSelection, writeSelection } from './selection.ts';

const savedStorage = globalThis.localStorage;
afterEach(() => {
  globalThis.localStorage = savedStorage;
});

function storage() {
  const values = new Map();
  globalThis.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
  };
  return values;
}

test('stores the selection', () => {
  const values = storage();
  const selected = { modelSource: 'channel', selectedChannel: 'second', model: 'model-b' };
  writeSelection(selected);
  assert.deepEqual(readSelection(), selected);
  assert.deepEqual(JSON.parse(values.get('axonhub_playground_selection')), selected);
});

test('corrupt and unavailable storage fall back without throwing', () => {
  const values = storage();
  values.set('axonhub_playground_selection', '{broken');
  assert.deepEqual(readSelection(), defaultSelection);
  values.set('axonhub_playground_selection', JSON.stringify({ modelSource: 'unknown', model: 'a' }));
  assert.deepEqual(readSelection(), defaultSelection);
  globalThis.localStorage = {
    getItem: () => {
      throw new Error('denied');
    },
    setItem: () => {
      throw new Error('denied');
    },
  };
  assert.deepEqual(readSelection(), defaultSelection);
  assert.doesNotThrow(() => writeSelection(defaultSelection));
});

const channels = [
  { value: 'first', models: ['first-a'] },
  { value: 'second', models: ['second-a', 'second-b'] },
];

test('restores a valid channel and model and falls back when either disappears', () => {
  const selected = { modelSource: 'channel', selectedChannel: 'second', model: 'second-b' };
  assert.deepEqual(resolveSelection(selected, channels, [], false), selected);
  assert.deepEqual(resolveSelection({ ...selected, model: 'deleted' }, channels, [], false), { ...selected, model: 'second-a' });
  assert.deepEqual(resolveSelection(selected, channels.slice(0, 1), [], false), {
    modelSource: 'channel',
    selectedChannel: 'first',
    model: 'first-a',
  });
  assert.deepEqual(resolveSelection(selected, [], [], false), defaultSelection);
});

test('gateway model survives with access and falls back without access', () => {
  const selected = { modelSource: 'model_gateway', selectedChannel: 'second', model: 'gateway-b' };
  assert.deepEqual(resolveSelection(selected, channels, ['gateway-a', 'gateway-b'], true), selected);
  assert.deepEqual(resolveSelection(selected, channels, ['gateway-a'], true), { ...selected, model: 'gateway-a' });
  assert.deepEqual(resolveSelection(selected, channels, ['gateway-a'], false), {
    modelSource: 'channel',
    selectedChannel: 'second',
    model: 'second-a',
  });
});
