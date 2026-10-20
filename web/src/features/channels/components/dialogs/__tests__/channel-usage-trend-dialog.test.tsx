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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { getChannelUsageTrend } from '../../../api'
import type { Channel } from '../../../types'
import { ChannelUsageTrendDialog } from '../channel-usage-trend-dialog'

vi.mock('../../../api', () => ({ getChannelUsageTrend: vi.fn() }))

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

const channel = { id: 9, name: 'cc2' } as Channel

function response(tokens: number) {
  return {
    success: true,
    data: {
      channel_id: 9,
      start_timestamp: 0,
      end_timestamp: 0,
      granularity: 'hour' as const,
      buckets: [
        { bucket: 3600, token_used: tokens, quota: 500000, count: 3 },
        { bucket: 7200, token_used: 0, quota: 0, count: 0 },
      ],
      total_tokens: tokens,
      total_quota: 500000,
      total_count: 3,
    },
  }
}

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <ChannelUsageTrendDialog channel={channel} onClose={() => {}} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

test('loads the last 24 hours and shows the totals', async () => {
  vi.mocked(getChannelUsageTrend).mockResolvedValue(response(1234567))
  renderDialog()

  expect(await screen.findByText('1,234,567')).toBeTruthy()
  const call = vi.mocked(getChannelUsageTrend).mock.calls[0]
  expect(call[0]).toBe(9)
  expect(call[1].end_timestamp - call[1].start_timestamp).toBe(24 * 3600)
  expect(call[1].tz_offset).toBe(-new Date().getTimezoneOffset() || 0)
})

test('picking another range queries that range', async () => {
  vi.mocked(getChannelUsageTrend).mockResolvedValue(response(10))
  renderDialog()
  await screen.findByText('Last 24 hours')

  await userEvent.click(screen.getByRole('button', { name: 'Last 7 days' }))

  await waitFor(() => {
    const last = vi.mocked(getChannelUsageTrend).mock.calls.at(-1)
    expect(last?.[1].end_timestamp).toBeGreaterThan(0)
    expect(
      (last?.[1].end_timestamp ?? 0) - (last?.[1].start_timestamp ?? 0)
    ).toBe(7 * 24 * 3600)
  })
})

test('shows an empty state when the period has no consumption', async () => {
  vi.mocked(getChannelUsageTrend).mockResolvedValue(response(0))
  renderDialog()

  expect(await screen.findByText('No consumption in this period')).toBeTruthy()
})

test('an invalid custom range is reported and not queried', async () => {
  vi.mocked(getChannelUsageTrend).mockResolvedValue(response(10))
  renderDialog()
  await screen.findByText('Last 24 hours')

  await userEvent.click(screen.getByRole('button', { name: 'Custom' }))
  await waitFor(() => {
    expect(vi.mocked(getChannelUsageTrend).mock.calls.length).toBe(2)
  })

  await userEvent.clear(screen.getByLabelText('End date'))

  expect(
    await screen.findByText('Choose a valid start and end date')
  ).toBeTruthy()
  expect(vi.mocked(getChannelUsageTrend).mock.calls.length).toBe(2)
})
