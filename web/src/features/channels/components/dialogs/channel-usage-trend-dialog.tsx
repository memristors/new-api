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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import dayjs from '@/lib/dayjs'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'

import { getChannelUsageTrend } from '../../api'
import {
  channelsQueryKeys,
  getViewerTzOffsetMinutes,
  resolveCustomRange,
  resolvePresetRange,
  sumUsageBuckets,
  USAGE_TREND_PRESETS,
  type UsageTrendPresetKey,
  type UsageTrendRange,
} from '../../lib'
import type { Channel } from '../../types'

type ChannelUsageTrendDialogProps = {
  channel: Channel
  onClose: () => void
}

type PresetSelection = UsageTrendPresetKey | 'custom'

const DEFAULT_PRESET_HOURS = 24

function toDayInputValue(unixSeconds: number): string {
  return dayjs.unix(unixSeconds).format('YYYY-MM-DD')
}

export function ChannelUsageTrendDialog(props: ChannelUsageTrendDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const [preset, setPreset] = useState<PresetSelection>('24h')
  const [presetRange, setPresetRange] = useState<UsageTrendRange>(() =>
    resolvePresetRange(DEFAULT_PRESET_HOURS)
  )
  const [startDay, setStartDay] = useState(() =>
    toDayInputValue(presetRange.start)
  )
  const [endDay, setEndDay] = useState(() => toDayInputValue(presetRange.end))

  const customRange = useMemo(
    () => resolveCustomRange(startDay, endDay),
    [startDay, endDay]
  )
  const range = preset === 'custom' ? customRange : presetRange
  const tzOffset = getViewerTzOffsetMinutes()

  const query = useQuery({
    queryKey: channelsQueryKeys.usageTrend(props.channel.id, {
      start: range?.start,
      end: range?.end,
      tzOffset,
    }),
    enabled: range !== null,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      if (!range) return null
      const res = await getChannelUsageTrend(props.channel.id, {
        start_timestamp: range.start,
        end_timestamp: range.end,
        tz_offset: tzOffset,
      })
      if (!res.success || !res.data) {
        throw createServerError(res, t('Failed to load consumption trend'))
      }
      return res.data
    },
  })

  const data = query.data
  const queryError = query.error
  useEffect(() => {
    if (queryError) {
      handleServerError(queryError, t('Failed to load consumption trend'))
    }
  }, [queryError, t])

  const totals = useMemo(
    () => sumUsageBuckets(data?.buckets ?? []),
    [data?.buckets]
  )
  const isHourly = data?.granularity === 'hour'

  const chartData = useMemo(
    () =>
      (data?.buckets ?? []).map((bucket) => ({
        label: dayjs
          .unix(bucket.bucket)
          .format(isHourly ? 'MM-DD HH:00' : 'MM-DD'),
        token_used: bucket.token_used,
      })),
    [data?.buckets, isHourly]
  )

  const chartConfig = {
    token_used: { label: t('Tokens'), color: 'var(--chart-1)' },
  } satisfies ChartConfig

  const handleSelectPreset = (key: UsageTrendPresetKey, hours: number) => {
    const next = resolvePresetRange(hours)
    setPreset(key)
    setPresetRange(next)
    setStartDay(toDayInputValue(next.start))
    setEndDay(toDayInputValue(next.end))
  }

  const handleRefresh = () => {
    const selected = USAGE_TREND_PRESETS.find((item) => item.key === preset)
    if (selected) {
      setPresetRange(resolvePresetRange(selected.hours))
      return
    }
    query.refetch()
  }

  const isLoading = range !== null && query.isPending
  const showEmpty = !isLoading && data != null && totals.tokens === 0
  const showChart = !isLoading && !showEmpty && data != null
  const rangeInvalid = preset === 'custom' && customRange === null

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Consumption Trend')}
      description={
        <>
          {t('Token consumption of channel:')}{' '}
          <strong>{props.channel.name}</strong>
        </>
      }
      contentClassName='sm:max-w-3xl'
      bodyClassName='space-y-4'
      footer={
        <Button variant='outline' onClick={props.onClose}>
          {t('Close')}
        </Button>
      }
    >
      <div className='flex flex-wrap items-center gap-2'>
        {USAGE_TREND_PRESETS.map((item) => (
          <Button
            key={item.key}
            size='sm'
            variant={preset === item.key ? 'default' : 'outline'}
            aria-pressed={preset === item.key}
            onClick={() => handleSelectPreset(item.key, item.hours)}
          >
            {t(item.label)}
          </Button>
        ))}
        <Button
          size='sm'
          variant={preset === 'custom' ? 'default' : 'outline'}
          aria-pressed={preset === 'custom'}
          onClick={() => setPreset('custom')}
        >
          {t('Custom')}
        </Button>
        <Button
          size='sm'
          variant='ghost'
          className='ms-auto'
          disabled={query.isFetching}
          onClick={handleRefresh}
        >
          {t('Refresh')}
        </Button>
      </div>

      {preset === 'custom' && (
        <div className='flex flex-wrap items-center gap-2 text-sm'>
          <input
            type='date'
            aria-label={t('Start date')}
            className='border-input bg-background h-8 rounded-md border px-2'
            value={startDay}
            max={endDay || undefined}
            onChange={(event) => setStartDay(event.target.value)}
          />
          <span className='text-muted-foreground'>{t('to')}</span>
          <input
            type='date'
            aria-label={t('End date')}
            className='border-input bg-background h-8 rounded-md border px-2'
            value={endDay}
            min={startDay || undefined}
            onChange={(event) => setEndDay(event.target.value)}
          />
        </div>
      )}

      {rangeInvalid && (
        <p className='text-destructive text-sm'>
          {t('Choose a valid start and end date')}
        </p>
      )}

      <div className='grid grid-cols-3 gap-3'>
        <SummaryItem
          label={t('Tokens')}
          value={formatNumber(totals.tokens, locale)}
        />
        <SummaryItem
          label={t('Requests')}
          value={formatNumber(totals.count, locale)}
        />
        <SummaryItem label={t('Quota')} value={formatQuota(totals.quota)} />
      </div>

      {isLoading && <Skeleton className='h-64 w-full' />}

      {showEmpty && (
        <div className='text-muted-foreground flex h-64 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No consumption in this period')}
        </div>
      )}

      {showChart && (
        <ChartContainer config={chartConfig} className='h-64 w-full'>
          <BarChart data={chartData} margin={{ left: 4, right: 4 }}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey='label'
              tickLine={false}
              axisLine={false}
              minTickGap={24}
              tickMargin={8}
            />
            <YAxis
              width={48}
              tickLine={false}
              axisLine={false}
              tickFormatter={(value: number) =>
                formatCompactNumber(value, locale)
              }
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value) => formatNumber(Number(value), locale)}
                />
              }
            />
            <Bar
              dataKey='token_used'
              fill='var(--color-token_used)'
              radius={2}
            />
          </BarChart>
        </ChartContainer>
      )}

      {data != null && (
        <p className='text-muted-foreground text-xs'>
          {isHourly
            ? t('Hourly buckets, shown in your local time.')
            : t('Daily buckets, shown in your local time.')}{' '}
          {t('Cache read and write tokens are included.')}
        </p>
      )}
    </Dialog>
  )
}

function SummaryItem(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/50 rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 text-lg font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}
