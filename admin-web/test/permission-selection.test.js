import { expect, test } from 'vitest'
import {
  moduleCheckState,
  selectedControlledCodes,
} from '../src/permission-selection.js'

const modules = [
  { code: 'radar.monitored', accessMode: 'public' },
  { code: 'radar.main_strategy', accessMode: 'user_allowlist' },
  { code: 'radar.blue_strategy', accessMode: 'user_allowlist' },
]

test('module state is checked only when every selected user has the grant', () => {
  expect(moduleCheckState([
    { userId: 'a', moduleCodes: ['radar.main_strategy'] },
    { userId: 'b', moduleCodes: ['radar.main_strategy'] },
  ], 'radar.main_strategy')).toBe('checked')

  expect(moduleCheckState([
    { userId: 'a', moduleCodes: ['radar.main_strategy'] },
    { userId: 'b', moduleCodes: [] },
  ], 'radar.main_strategy')).toBe('indeterminate')

  expect(moduleCheckState([
    { userId: 'a', moduleCodes: [] },
    { userId: 'b', moduleCodes: [] },
  ], 'radar.main_strategy')).toBe('unchecked')
})

test('public modules cannot enter the save payload', () => {
  expect(selectedControlledCodes(modules, [
    'radar.monitored',
    'radar.blue_strategy',
  ])).toEqual(['radar.blue_strategy'])
})
