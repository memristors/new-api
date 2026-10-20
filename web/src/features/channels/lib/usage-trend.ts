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
import type { ChannelUsageBucket } from '../types'

export const USAGE_TREND_PRESETS = [
  { key: '24h', hours: 24, label: 'Last 24 hours' },
  { key: '3d', hours: 72, label: 'Last 3 days' },
  { key: '7d', hours: 24 * 7, label: 'Last 7 days' },
  { key: '30d', hours: 24 * 30, label: 'Last 30 days' },
  { key: '90d', hours: 24 * 90, label: 'Last 90 days' },
] as const

export type UsageTrendPresetKey = (typeof USAGE_TREND_PRESETS)[number]['key']

export type UsageTrendRange = {
  /** Unix seconds */
  start: number
  /** Unix seconds */
  end: number
}

/** The viewer's UTC offset in minutes, east positive (UTC+8 -> 480). */
export function getViewerTzOffsetMinutes(date: Date = new Date()): number {
  // getTimezoneOffset() is minutes *behind* UTC; `|| 0` also normalises -0
  return -date.getTimezoneOffset() || 0
}

/** Range ending now. The end is rounded down to the minute so the query key stays stable. */
export function resolvePresetRange(
  hours: number,
  nowMs: number = Date.now()
): UsageTrendRange {
  const end = Math.floor(nowMs / 60000) * 60
  return { start: end - hours * 3600, end }
}

function parseLocalDate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return null
  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const date = new Date(year, month - 1, day)
  const valid =
    date.getFullYear() === year &&
    date.getMonth() === month - 1 &&
    date.getDate() === day
  return valid ? date : null
}

/**
 * Range from two local calendar days (YYYY-MM-DD), both inclusive: it runs from the start of
 * the first day to the last second of the second day. Returns null for unparsable or
 * reversed input.
 */
export function resolveCustomRange(
  startDay: string,
  endDay: string
): UsageTrendRange | null {
  const startDate = parseLocalDate(startDay)
  const endDate = parseLocalDate(endDay)
  if (!startDate || !endDate) return null
  const nextDay = new Date(
    endDate.getFullYear(),
    endDate.getMonth(),
    endDate.getDate() + 1
  )
  const start = Math.floor(startDate.getTime() / 1000)
  const end = Math.floor(nextDay.getTime() / 1000) - 1
  return start < end ? { start, end } : null
}

export type UsageTotals = {
  tokens: number
  quota: number
  count: number
  input: number
  output: number
  cacheRead: number
  cacheWrite: number
}

export function sumUsageBuckets(buckets: ChannelUsageBucket[]): UsageTotals {
  return buckets.reduce<UsageTotals>(
    (sum, bucket) => ({
      tokens: sum.tokens + bucket.token_used,
      quota: sum.quota + bucket.quota,
      count: sum.count + bucket.count,
      input: sum.input + (bucket.input_tokens || 0),
      output: sum.output + (bucket.output_tokens || 0),
      cacheRead: sum.cacheRead + (bucket.cache_read_tokens || 0),
      cacheWrite: sum.cacheWrite + (bucket.cache_write_tokens || 0),
    }),
    {
      tokens: 0,
      quota: 0,
      count: 0,
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
    }
  )
}

/**
 * Token amount for people: small values keep every digit, large ones use the locale's own
 * units (138.89M in English, 1.39亿 in Chinese) so the magnitude reads at a glance.
 */
export function formatTokenAmount(value: number, locale?: string): string {
  if (!Number.isFinite(value)) return '-'
  if (Math.abs(value) < 10000) {
    return new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(
      value
    )
  }
  return new Intl.NumberFormat(locale, {
    notation: 'compact',
    maximumFractionDigits: 2,
  }).format(value)
}

/** Share of `part` in `whole` as a percentage string, e.g. "93.4%"; "-" without a whole. */
export function formatShare(
  part: number,
  whole: number,
  locale?: string
): string {
  if (!whole) return '-'
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    maximumFractionDigits: 1,
  }).format(part / whole)
}
