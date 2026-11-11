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
import { useQuery } from '@tanstack/react-query'
import { useDeferredValue, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { getModelAvailability, type AvailabilityHours } from './api'
import { AvailabilityTable } from './components/availability-table'

const RANGES = [24, 72, 168] as const

export function ModelAvailability() {
  const { t } = useTranslation()
  const [hours, setHours] = useState<AvailabilityHours>(24)
  const [search, setSearch] = useState('')
  const filter = useDeferredValue(search.trim().toLowerCase())
  const query = useQuery({
    queryKey: ['model-availability', hours],
    queryFn: ({ signal }) => getModelAvailability(hours, signal),
    staleTime: 30_000,
    refetchInterval: 60_000,
    retry: false,
  })
  const filtered = useMemo(() => {
    if (!query.data) return undefined
    return {
      ...query.data,
      models: query.data.models.filter((model) =>
        model.model_name.toLowerCase().includes(filter)
      ),
    }
  }, [query.data, filter])

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Hourly model availability')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <div className='flex shrink-0 flex-wrap items-center justify-between gap-4 rounded-xl border-[0.5px] px-4 py-4'>
            <p className='text-muted-foreground max-w-2xl text-sm'>
              {t(
                'Availability is successful requests divided by completed requests. The current hour is still in progress; hours without requests show no data.'
              )}
            </p>
            <div className='flex w-full flex-wrap items-center gap-2 sm:w-auto'>
              <Input
                type='search'
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t('Search models')}
                aria-label={t('Search models')}
                className='h-9 w-full rounded-full sm:w-56'
              />
              <div
                role='group'
                aria-label={t('Time range')}
                className='flex items-center gap-1.5'
              >
                {RANGES.map((range) => (
                  <Button
                    key={range}
                    variant={hours === range ? 'default' : 'outline'}
                    className='h-9 rounded-full px-3'
                    aria-pressed={hours === range}
                    onClick={() => setHours(range)}
                  >
                    {range === 168
                      ? t('7 days')
                      : t('{{count}} hours', { count: range })}
                  </Button>
                ))}
              </div>
            </div>
          </div>
          {query.isPending && <LoadingState />}
          {query.isError && <ErrorState onRetry={() => void query.refetch()} />}
          {!query.isPending &&
            !query.isError &&
            filtered?.models.length === 0 && (
              <EmptyState
                title={
                  filter
                    ? t('No models found')
                    : t('No performance data available')
                }
              />
            )}
          {!query.isError && filtered != null && filtered.models.length > 0 && (
            <AvailabilityTable data={filtered} />
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
