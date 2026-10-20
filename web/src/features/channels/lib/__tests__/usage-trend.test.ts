/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import type { ChannelUsageBucket } from '../../types'
import {
  getViewerTzOffsetMinutes,
  resolveCustomRange,
  resolvePresetRange,
  sumUsageBuckets,
} from '../usage-trend'

describe('resolvePresetRange', () => {
  test('ends at the current minute and spans the requested hours', () => {
    const nowMs = Date.UTC(2026, 9, 20, 10, 31, 42)
    const range = resolvePresetRange(24, nowMs)
    expect(range.end).toBe(Date.UTC(2026, 9, 20, 10, 31, 0) / 1000)
    expect(range.end - range.start).toBe(24 * 3600)
  })
})

describe('resolveCustomRange', () => {
  test('covers both selected local days completely', () => {
    const range = resolveCustomRange('2026-10-01', '2026-10-03')
    expect(range).not.toBeNull()
    expect(range?.start).toBe(new Date(2026, 9, 1).getTime() / 1000)
    expect(range?.end).toBe(new Date(2026, 9, 4).getTime() / 1000 - 1)
  })

  test('accepts a single day', () => {
    const range = resolveCustomRange('2026-10-01', '2026-10-01')
    expect(range).not.toBeNull()
    expect((range?.end ?? 0) - (range?.start ?? 0)).toBeGreaterThan(23 * 3600)
  })

  test('rejects reversed, empty and impossible dates', () => {
    expect(resolveCustomRange('2026-10-05', '2026-10-01')).toBeNull()
    expect(resolveCustomRange('', '2026-10-01')).toBeNull()
    expect(resolveCustomRange('2026-10-01', '')).toBeNull()
    expect(resolveCustomRange('2026-02-31', '2026-03-01')).toBeNull()
    expect(resolveCustomRange('not-a-date', '2026-03-01')).toBeNull()
  })
})

describe('getViewerTzOffsetMinutes', () => {
  test('is the negated JS offset, so east of UTC is positive', () => {
    const date = new Date(2026, 9, 20)
    expect(getViewerTzOffsetMinutes(date)).toBe(-date.getTimezoneOffset() || 0)
  })
})

describe('sumUsageBuckets', () => {
  test('adds tokens, quota and request counts', () => {
    const buckets: ChannelUsageBucket[] = [
      { bucket: 0, token_used: 10, quota: 2, count: 1 },
      { bucket: 3600, token_used: 0, quota: 0, count: 0 },
      { bucket: 7200, token_used: 30, quota: 5, count: 4 },
    ]
    expect(sumUsageBuckets(buckets)).toEqual({ tokens: 40, quota: 7, count: 5 })
    expect(sumUsageBuckets([])).toEqual({ tokens: 0, quota: 0, count: 0 })
  })
})
