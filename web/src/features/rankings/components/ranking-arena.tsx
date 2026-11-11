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
import { Link } from '@tanstack/react-router'
import { ArrowUpRight, Trophy } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ModelIdentity } from '@/components/model-identity'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { useModelCustomizations } from '@/hooks/use-model-customizations'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { RankingsSnapshot } from '../types'
import { GrowthText } from './growth-text'
import { UsageChart } from './usage-chart'

export function RankingArena(props: { snapshot: RankingsSnapshot }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [search, setSearch] = useState('')
  const [vendor, setVendor] = useState('')
  const custom = useModelCustomizations()
  const rows = useMemo(
    () =>
      props.snapshot.models.filter(
        (model) =>
          (!vendor || model.vendor === vendor) &&
          `${model.model_name} ${custom[model.model_name]?.display_name || ''}`
            .toLowerCase()
            .includes(search.toLowerCase().trim())
      ),
    [props.snapshot.models, vendor, search, custom]
  )
  const leaders = rows.slice(0, 3)
  const total =
    search || vendor
      ? rows.reduce((sum, model) => sum + model.total_tokens, 0)
      : (props.snapshot.total_tokens ??
        rows.reduce((sum, model) => sum + model.total_tokens, 0))
  const filteredModels = useMemo(() => {
    if (!search && !vendor) return undefined
    return new Set(
      props.snapshot.models_history.models
        .filter(
          (model) =>
            (!vendor || model.vendor === vendor) &&
            `${model.name} ${custom[model.name]?.display_name || ''}`
              .toLowerCase()
              .includes(search.trim().toLowerCase())
        )
        .map((model) => model.name)
    )
  }, [props.snapshot.models_history.models, vendor, search, custom])

  return (
    <div className='space-y-8'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div
          className='flex flex-wrap gap-1.5'
          role='group'
          aria-label={t('Vendor')}
        >
          <Button
            variant={!vendor ? 'secondary' : 'ghost'}
            aria-pressed={!vendor}
            onClick={() => setVendor('')}
          >
            {t('All')}
          </Button>
          {props.snapshot.vendors.map((entry) => (
            <Button
              key={entry.vendor}
              variant={vendor === entry.vendor ? 'secondary' : 'ghost'}
              aria-pressed={vendor === entry.vendor}
              onClick={() => setVendor(entry.vendor)}
            >
              {entry.vendor}
            </Button>
          ))}
        </div>
        <Input
          type='search'
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search models')}
          aria-label={t('Search models')}
          className='w-full sm:w-64'
        />
      </div>
      <div className='flex flex-wrap gap-x-10 gap-y-4 text-sm'>
        <div>
          <span className='text-muted-foreground'>{t('Total tokens')}</span>
          <p className='mt-1 text-xl font-semibold tabular-nums'>
            {formatCompactNumber(total, locale)}
          </p>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Active models')}</span>
          <p className='mt-1 text-xl font-semibold tabular-nums'>
            {formatNumber(
              vendor || search
                ? rows.length
                : (props.snapshot.total_models ?? rows.length),
              locale
            )}
          </p>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Vendors')}</span>
          <p className='mt-1 text-xl font-semibold tabular-nums'>
            {formatNumber(vendor ? 1 : props.snapshot.vendors.length, locale)}
          </p>
        </div>
      </div>
      {leaders.length > 0 && (
        <section
          className='grid grid-cols-2 gap-3 sm:grid-cols-3'
          aria-label={t('Leading models')}
        >
          {leaders.map((model, index) => (
            <Link
              key={model.model_name}
              to='/pricing/$modelId'
              params={{ modelId: model.model_name }}
              className={cn(
                'glass-surface group relative flex min-w-0 flex-col gap-6 overflow-hidden rounded-2xl p-5 transition-colors hover:bg-muted/80 sm:p-6',
                index === 0
                  ? 'bg-emerald-50 max-sm:col-span-2 dark:bg-emerald-950/70'
                  : 'bg-muted/40'
              )}
            >
              <div className='flex items-center justify-between'>
                <span
                  className={cn(
                    'text-4xl font-light tracking-tight tabular-nums',
                    index === 0
                      ? 'text-emerald-700 dark:text-emerald-300'
                      : 'text-muted-foreground/60'
                  )}
                >
                  {model.rank}
                </span>
                {index === 0 ? (
                  <Trophy
                    className='size-6 text-emerald-600 dark:text-emerald-400'
                    aria-hidden='true'
                  />
                ) : (
                  <ArrowUpRight
                    className='text-muted-foreground size-5'
                    aria-hidden='true'
                  />
                )}
              </div>
              <div className='min-w-0 space-y-3'>
                <ModelIdentity
                  name={model.model_name}
                  icon={model.vendor_icon}
                  size={28}
                  showId
                />
                <p className='text-muted-foreground text-xs'>{model.vendor}</p>
              </div>
              <div className='mt-auto flex items-end justify-between gap-2'>
                <div>
                  <span className='text-xl font-semibold tabular-nums'>
                    {formatCompactNumber(model.total_tokens, locale)}
                  </span>
                  <span className='text-muted-foreground ml-2 text-xs'>
                    {t('tokens')}
                  </span>
                </div>
                <GrowthText value={model.growth_pct} />
              </div>
            </Link>
          ))}
        </section>
      )}
      <UsageChart
        history={props.snapshot.models_history}
        models={filteredModels}
      />
      <Tabs defaultValue='models' className='space-y-5'>
        <div className='flex flex-wrap items-center justify-between gap-4'>
          <TabsList>
            <TabsTrigger value='models'>{t('Top Models')}</TabsTrigger>
            <TabsTrigger value='vendors'>{t('Vendors')}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value='models' className='space-y-4'>
          {rows.length === 0 ? (
            <EmptyState title={t('No models found')} />
          ) : (
            <ol className='space-y-1'>
              {rows.map((model) => (
                <li
                  key={model.model_name}
                  className='group hover:bg-muted/40 grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-3 rounded-xl px-3 py-4 sm:grid-cols-[2rem_minmax(0,1fr)_minmax(80px,30%)_auto] sm:px-5'
                >
                  <span className='text-muted-foreground text-sm tabular-nums'>
                    {model.rank}
                  </span>
                  <Link
                    to='/pricing/$modelId'
                    params={{ modelId: model.model_name }}
                    className='min-w-0 text-sm font-medium'
                  >
                    <ModelIdentity
                      name={model.model_name}
                      icon={model.vendor_icon}
                      showId
                    />
                  </Link>
                  <div
                    className='bg-muted hidden h-2 rounded-full sm:block'
                    aria-hidden='true'
                  >
                    <div
                      className='h-full rounded-full bg-emerald-500/70 transition-[width] duration-300 motion-reduce:transition-none'
                      style={{
                        width: `${Math.max(1, (model.total_tokens / (leaders[0]?.total_tokens || 1)) * 100)}%`,
                      }}
                    />
                  </div>
                  <div className='min-w-20 text-right'>
                    <p className='text-sm font-medium tabular-nums'>
                      {formatCompactNumber(model.total_tokens, locale)}
                    </p>
                    <GrowthText value={model.growth_pct} />
                  </div>
                </li>
              ))}
            </ol>
          )}
        </TabsContent>
        <TabsContent value='vendors'>
          <ol className='grid gap-3 sm:grid-cols-2'>
            {props.snapshot.vendors
              .filter((entry) => !vendor || entry.vendor === vendor)
              .map((entry) => (
                <li
                  key={entry.vendor}
                  className='glass-surface bg-muted/30 flex items-center gap-4 rounded-xl p-5'
                >
                  <span className='text-muted-foreground text-xl tabular-nums'>
                    {entry.rank}
                  </span>
                  <div className='min-w-0 flex-1'>
                    <Link
                      to='/pricing'
                      search={{ vendor: entry.vendor }}
                      className='text-sm font-semibold'
                    >
                      {entry.vendor}
                    </Link>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {formatNumber(entry.models_count, locale)} {t('Models')}
                    </p>
                  </div>
                  <div className='text-right'>
                    <p className='text-sm font-semibold tabular-nums'>
                      {(entry.share * 100).toFixed(1)}%
                    </p>
                    <GrowthText value={entry.growth_pct} />
                  </div>
                </li>
              ))}
          </ol>
        </TabsContent>
      </Tabs>
    </div>
  )
}
