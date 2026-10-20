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
import { formatNumber, formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'

import { getChannelUsageTrend } from '../../api'
import {
  channelsQueryKeys,
  formatShare,
  formatTokenAmount,
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
const BREAKDOWN_SERIES = [
  'input',
  'output',
  'cache_read',
  'cache_write',
] as const

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
        input: bucket.input_tokens,
        output: bucket.output_tokens,
        cache_read: bucket.cache_read_tokens,
        cache_write: bucket.cache_write_tokens,
      })),
    [data?.buckets, isHourly]
  )

  const breakdownTotal =
    totals.input + totals.output + totals.cacheRead + totals.cacheWrite
  const hasBreakdown = breakdownTotal > 0

  const chartConfig = {
    token_used: { label: t('Tokens'), color: 'var(--chart-1)' },
    input: { label: t('Input'), color: 'var(--chart-1)' },
    output: { label: t('Output'), color: 'var(--chart-2)' },
    cache_read: { label: t('Cache read'), color: 'var(--chart-3)' },
    cache_write: { label: t('Cache write'), color: 'var(--chart-4)' },
  } satisfies ChartConfig

  const breakdownItems = [
    { key: 'input', label: t('Input'), value: totals.input },
    { key: 'output', label: t('Output'), value: totals.output },
    { key: 'cache_read', label: t('Cache read'), value: totals.cacheRead },
    { key: 'cache_write', label: t('Cache write'), value: totals.cacheWrite },
  ] as const

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
          label={t('Total tokens')}
          value={formatTokenAmount(totals.tokens, locale)}
          hint={formatNumber(totals.tokens, locale)}
        />
        <SummaryItem
          label={t('Requests')}
          value={formatTokenAmount(totals.count, locale)}
          hint={formatNumber(totals.count, locale)}
        />
        <SummaryItem label={t('Quota')} value={formatQuota(totals.quota)} />
      </div>

      {hasBreakdown && (
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          {breakdownItems.map((item) => (
            <SummaryItem
              key={item.key}
              label={item.label}
              value={formatTokenAmount(item.value, locale)}
              hint={`${formatNumber(item.value, locale)} · ${formatShare(item.value, breakdownTotal, locale)}`}
              color={`var(--color-${item.key})`}
            />
          ))}
        </div>
      )}

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
              width={56}
              tickLine={false}
              axisLine={false}
              tickFormatter={(value: number) =>
                formatTokenAmount(value, locale)
              }
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value, name, item) => (
                    <div className='flex w-full items-center justify-between gap-4'>
                      <span className='flex items-center gap-1.5'>
                        <span
                          className='h-2 w-2 rounded-[2px]'
                          style={{ backgroundColor: item.color }}
                        />
                        {chartConfig[name as keyof typeof chartConfig]?.label ??
                          name}
                      </span>
                      <span className='font-mono tabular-nums'>
                        {formatTokenAmount(Number(value), locale)}
                        {Number(value) >= 10000 && (
                          <span className='text-muted-foreground'>
                            {' '}
                            ({formatNumber(Number(value), locale)})
                          </span>
                        )}
                      </span>
                    </div>
                  )}
                />
              }
            />
            {hasBreakdown ? (
              BREAKDOWN_SERIES.map((key) => (
                <Bar
                  key={key}
                  dataKey={key}
                  stackId='tokens'
                  fill={`var(--color-${key})`}
                />
              ))
            ) : (
              <Bar
                dataKey='token_used'
                fill='var(--color-token_used)'
                radius={2}
              />
            )}
          </BarChart>
        </ChartContainer>
      )}

      {data != null && (
        <p className='text-muted-foreground text-xs'>
          {isHourly
            ? t('Hourly buckets, shown in your local time.')
            : t('Daily buckets, shown in your local time.')}{' '}
          {t('Total tokens include cache reads and writes.')}
        </p>
      )}

      {hasBreakdown && breakdownTotal !== totals.tokens && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'The input/output/cache split comes from usage logs and may not cover the whole period.'
          )}
        </p>
      )}
    </Dialog>
  )
}

function SummaryItem(props: {
  label: string
  value: string
  hint?: string
  color?: string
}) {
  return (
    <div className='bg-muted/50 rounded-lg border p-3'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
        {props.color && (
          <span
            className='h-2 w-2 rounded-[2px]'
            style={{ backgroundColor: props.color }}
          />
        )}
        {props.label}
      </div>
      <div className='mt-1 text-lg font-semibold tabular-nums'>
        {props.value}
      </div>
      {props.hint && (
        <div className='text-muted-foreground text-xs tabular-nums'>
          {props.hint}
        </div>
      )}
    </div>
  )
}
