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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { ModelIdentity } from '@/components/model-identity'
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getSuccessRateLevel,
  type SuccessRateLevel,
} from '@/features/performance-metrics/lib/format'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { AvailabilityData } from '../api'

const CELL_TONES: Record<SuccessRateLevel, string> = {
  excellent:
    'bg-emerald-100 text-emerald-900 dark:bg-emerald-900 dark:text-emerald-200',
  good: 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
  warning: 'bg-amber-100 text-amber-900 dark:bg-yellow-950 dark:text-amber-200',
  critical: 'bg-red-100 text-red-900 dark:bg-red-950 dark:text-red-200',
  unknown: 'bg-background text-muted-foreground',
}

export function AvailabilityTable(props: { data: AvailabilityData }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const hourFormatter = new Intl.DateTimeFormat(locale, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    hourCycle: 'h23',
  })
  const rateFormatter = new Intl.NumberFormat(locale, {
    style: 'percent',
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })
  const hours = useMemo(() => {
    const end = props.data.window_end - (props.data.window_end % 3600)
    const result: number[] = []
    for (let ts = props.data.window_start; ts <= end; ts += 3600) {
      result.push(ts)
    }
    return result
  }, [props.data.window_start, props.data.window_end])
  const rows = useMemo(
    () =>
      props.data.models.map((model) => ({
        name: model.model_name,
        icon: model.icon,
        points: new Map(model.series.map((point) => [point.ts, point])),
      })),
    [props.data.models]
  )

  return (
    <div className='isolate min-h-0 flex-1 overflow-hidden rounded-xl border-[0.5px]'>
      <StaticDataTable
        className='availability-table h-full overflow-auto rounded-none border-0 [--availability-model-width:calc(var(--spacing)*48)] sm:[--availability-model-width:calc(var(--spacing)*60)]'
        containerProps={{
          role: 'region',
          'aria-label': t('Hourly model availability'),
          tabIndex: 0,
        }}
        tableProps={{
          withContainer: false,
          'aria-label': t('Hourly model availability'),
          style: {
            width: `calc(var(--availability-model-width) + ${hours.length * 24} * var(--spacing))`,
          },
        }}
        tableClassName='table-fixed border-separate border-spacing-0 [&_td]:border-r-[0.5px] [&_td]:border-b-[0.5px] [&_th]:border-r-[0.5px] [&_th]:border-b-[0.5px]'
      >
        <TableHeader className='sticky top-0 z-20'>
          <TableRow className='hover:bg-transparent'>
            <TableHead
              scope='col'
              className='bg-muted sticky left-0 z-30 h-12 w-48 min-w-48 px-4 sm:w-60 sm:min-w-60'
            >
              {t('Model')}
            </TableHead>
            {hours.map((ts) => (
              <TableHead
                key={ts}
                scope='col'
                className='bg-muted h-12 w-24 px-3 text-center'
                title={
                  ts === hours.at(-1)
                    ? t('Current hour is still in progress')
                    : undefined
                }
              >
                {hourFormatter.format(ts * 1000)}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.name} className='hover:bg-transparent'>
              <TableHead
                scope='row'
                className='bg-background sticky left-0 z-10 h-14 max-w-48 px-4 sm:max-w-60'
                title={row.name}
              >
                <ModelIdentity name={row.name} icon={row.icon || row.name} />
              </TableHead>
              {hours.map((ts) => {
                const point = row.points.get(ts)
                const hasData = point != null && point.request_count > 0
                const rate = hasData
                  ? (point.success_count / point.request_count) * 100
                  : Number.NaN
                const description = hasData
                  ? t('{{successful}} of {{total}} requests succeeded', {
                      successful: formatNumber(point.success_count, locale),
                      total: formatNumber(point.request_count, locale),
                    })
                  : t('No Data')
                return (
                  <TableCell
                    key={ts}
                    data-availability-level={getSuccessRateLevel(rate)}
                    className={cn(
                      'h-14 px-3 py-2 text-center',
                      CELL_TONES[getSuccessRateLevel(rate)]
                    )}
                    title={description}
                    aria-label={`${hourFormatter.format(ts * 1000)}: ${description}`}
                  >
                    {hasData ? (
                      <>
                        <div>{rateFormatter.format(rate / 100)}</div>
                        <div
                          data-table-text='secondary'
                          className='mt-0.5 opacity-80'
                        >
                          {formatNumber(point.request_count, locale)}
                        </div>
                      </>
                    ) : (
                      <span aria-hidden='true'>—</span>
                    )}
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </StaticDataTable>
    </div>
  )
}
