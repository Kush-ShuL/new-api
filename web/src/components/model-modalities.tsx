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
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  MODEL_INPUT_MODALITY_OPTIONS,
  MODEL_MODALITY_OPTIONS,
} from '@/lib/model-modalities'
import { cn } from '@/lib/utils'

export function ModelModalities(props: {
  input?: readonly string[]
  output?: readonly string[]
  className?: string
  compact?: boolean
}) {
  const { t } = useTranslation()
  if (!props.input?.length && !props.output?.length) return null
  return (
    <div
      className={cn(
        'grid grid-cols-2 gap-4 py-1',
        props.compact && 'gap-3 py-0',
        props.className
      )}
      aria-label={t('Supported modalities')}
    >
      {[
        {
          label: t('Input'),
          values: props.input,
          options: MODEL_INPUT_MODALITY_OPTIONS,
        },
        {
          label: t('Output'),
          values: props.output,
          options: MODEL_MODALITY_OPTIONS,
        },
      ].map((group, index) => (
        <div
          key={group.label}
          className={cn(
            'min-w-0',
            props.compact ? 'flex flex-wrap items-center gap-1.5' : 'space-y-2',
            index === 1 && 'border-border/60 border-l-[0.5px] pl-4'
          )}
        >
          <p className='text-muted-foreground shrink-0 text-xs'>
            {group.label}
          </p>
          {!group.values?.length ? (
            <span className='text-muted-foreground text-xs'>
              {t('Not configured')}
            </span>
          ) : (
            <div
              className={cn(
                'flex flex-nowrap items-center gap-2',
                props.compact && 'gap-1.5'
              )}
            >
              {group.options
                .filter(
                  (modality, i) =>
                    i < 4 || group.values?.includes(modality.value)
                )
                .map((modality) => {
                  const supported = group.values?.includes(modality.value)
                  const label = `${group.label}: ${t(modality.label)} — ${supported ? t('Supported') : t('Not supported')}`
                  return (
                    <Tooltip key={modality.value}>
                      <TooltipTrigger
                        render={
                          <span
                            tabIndex={0}
                            aria-label={label}
                            className={cn(
                              'relative inline-flex size-4 shrink-0 items-center justify-center',
                              props.compact && 'size-3.5',
                              !supported && 'text-muted-foreground/35'
                            )}
                          />
                        }
                      >
                        <modality.icon
                          aria-hidden
                          className={props.compact ? 'size-3.5' : 'size-4'}
                          strokeWidth={1.8}
                        />
                        {!supported && (
                          <span
                            aria-hidden
                            className='absolute h-[1.5px] w-5 rotate-45 rounded-full bg-current'
                          />
                        )}
                      </TooltipTrigger>
                      <TooltipContent>{label}</TooltipContent>
                    </Tooltip>
                  )
                })}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
