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
import { VChart } from '@visactor/react-vchart'
import { useReducedMotion } from 'motion/react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useModelCustomizations } from '@/hooks/use-model-customizations'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatNumber } from '@/lib/format'
import { useChartTheme } from '@/lib/use-chart-theme'
import { CHART_TOOLTIP, VCHART_OPTION } from '@/lib/vchart'

import { buildUsageSeries, type UsageMetric } from '../lib/usage-series'
import type { ModelHistorySeries } from '../types'

export function UsageChart(props: {
  history: ModelHistorySeries
  models?: Set<string>
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const { resolvedTheme, themeReady } = useChartTheme()
  const reducedMotion = useReducedMotion()
  const [selectedMetric, setMetric] = useState<UsageMetric>('requests')
  const [hiddenModels, setHiddenModels] = useState<Set<string>>(() => new Set())
  const custom = useModelCustomizations()
  const names = useMemo(
    () => props.history.models.map((model) => model.name),
    [props.history.models]
  )
  const visibleNames = names.filter(
    (name) => !props.models || props.models.has(name)
  )
  const colors = useMemo(
    () =>
      names.map(
        (_, index) =>
          `hsl(${(index * 137.508) % 360} 65% ${resolvedTheme === 'dark' ? 65 : 42}%)`
      ),
    [names, resolvedTheme]
  )
  const hasRequests = props.history.points.some(
    (point) => point.requests != null
  )
  const metric = hasRequests ? selectedMetric : 'tokens'
  const points = useMemo(
    () =>
      buildUsageSeries(props.history, metric, props.models).filter(
        (point) => !hiddenModels.has(point.model)
      ),
    [props.history, metric, props.models, hiddenModels]
  )
  const total = points.reduce((sum, point) => sum + point.value, 0)
  const spec = useMemo(() => {
    const date = new Intl.DateTimeFormat(
      locale,
      props.history.bucket_seconds === 3600
        ? { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }
        : { month: 'short', day: 'numeric' }
    )
    const text = resolvedTheme === 'dark' ? '#a3a3a3' : '#737373'
    return {
      type: 'line' as const,
      background: 'transparent',
      data: [
        {
          id: 'usage',
          values: points.map((point) => ({
            ...point,
            timestamp: String(point.ts),
            label: date.format(point.ts * 1000),
          })),
        },
      ],
      xField: 'timestamp',
      yField: 'value',
      seriesField: 'model',
      color: { type: 'ordinal' as const, domain: names, range: colors },
      line: { style: { lineWidth: 2, curveType: 'monotone' } },
      point: { visible: props.history.buckets === 1, style: { size: 5 } },
      axes: [
        {
          orient: 'bottom',
          tick: { visible: false },
          domainLine: { visible: false },
          label: {
            autoHide: true,
            style: { fill: text, fontSize: 12 },
            formatMethod: (value: string | number) =>
              date.format(Number(value) * 1000),
          },
          grid: { visible: false },
        },
        {
          orient: 'left',
          min: 0,
          tick: { visible: false },
          domainLine: { visible: false },
          grid: {
            visible: true,
            style: {
              stroke: resolvedTheme === 'dark' ? '#303030' : '#eeeeee',
              lineDash: [3, 4],
            },
          },
          label: {
            style: { fill: text, fontSize: 12 },
            formatMethod: (value: number | string) =>
              formatCompactNumber(Number(value), locale),
          },
        },
      ],
      legends: { visible: false },
      tooltip: {
        ...CHART_TOOLTIP,
        mark: {
          title: {
            value: (datum: Record<string, unknown>) => String(datum.label),
          },
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                custom[String(datum.model)]?.display_name ||
                String(datum.model),
              value: (datum: Record<string, unknown>) =>
                formatNumber(Number(datum.value), locale),
            },
          ],
        },
      },
      animation: !reducedMotion,
      animationAppear: { duration: 350 },
    }
  }, [
    locale,
    props.history.bucket_seconds,
    props.history.buckets,
    resolvedTheme,
    points,
    reducedMotion,
    names,
    colors,
    custom,
  ])

  return (
    <Card
      role='region'
      className='gap-0 rounded-2xl px-4 py-5 sm:px-6'
      aria-label={t('Usage trend')}
    >
      <div className='mb-5 flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h2 className='text-sm font-semibold'>{t('Usage trend')}</h2>
          <p className='text-muted-foreground mt-1 text-xs'>
            {formatNumber(total, locale)}{' '}
            {metric === 'requests' ? t('Requests') : t('tokens')}
          </p>
        </div>
        <div role='group' aria-label={t('Usage metric')} className='flex gap-1'>
          <Button
            size='sm'
            variant={metric === 'requests' ? 'secondary' : 'ghost'}
            disabled={!hasRequests}
            aria-pressed={metric === 'requests'}
            onClick={() => setMetric('requests')}
          >
            {t('Requests')}
          </Button>
          <Button
            size='sm'
            variant={metric === 'tokens' ? 'secondary' : 'ghost'}
            aria-pressed={metric === 'tokens'}
            onClick={() => setMetric('tokens')}
          >
            {t('tokens')}
          </Button>
        </div>
      </div>
      <div
        className='mb-4 flex max-h-24 flex-wrap gap-x-3 gap-y-1 overflow-auto'
        role='group'
        aria-label={t('Models')}
      >
        {visibleNames.map((name) => (
          <Button
            key={name}
            size='sm'
            variant='ghost'
            aria-pressed={!hiddenModels.has(name)}
            className={
              hiddenModels.has(name) ? 'max-w-56 opacity-40' : 'max-w-56'
            }
            onClick={() =>
              setHiddenModels((current) => {
                const next = new Set(current)
                if (next.has(name)) {
                  next.delete(name)
                } else {
                  next.add(name)
                }
                return next
              })
            }
          >
            <span
              aria-hidden='true'
              className='size-2 shrink-0 rounded-full'
              style={{ backgroundColor: colors[names.indexOf(name)] }}
            />
            <span className='truncate text-xs'>
              {custom[name]?.display_name || name}
            </span>
          </Button>
        ))}
      </div>
      <div className='h-60' role='img' aria-label={t('Usage trend')}>
        {points.length === 0 && <EmptyState className='min-h-60' />}
        {points.length > 0 && !themeReady && (
          <Skeleton className='h-full w-full' />
        )}
        {points.length > 0 && themeReady && (
          <VChart spec={spec} options={VCHART_OPTION} />
        )}
      </div>
    </Card>
  )
}
