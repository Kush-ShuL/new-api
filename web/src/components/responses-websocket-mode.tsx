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
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export type ResponsesWebSocketMode = 'pool' | 'passthrough' | 'http_bridge'

export function ResponsesWebSocketModeSelect(props: {
  value: string
  onChange: (value: string) => void
  inherit?: boolean
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const options = [
    ...(props.inherit
      ? [{ value: 'inherit', label: t('Follow channel') }]
      : []),
    { value: 'pool', label: t('Connection pool') },
    { value: 'passthrough', label: t('Passthrough') },
    { value: 'http_bridge', label: t('HTTP bridge') },
  ]
  return (
    <RadioGroup
      value={props.value || 'inherit'}
      onValueChange={props.onChange}
      disabled={props.disabled}
      className='grid grid-cols-2 gap-3'
    >
      {options.map((option) => (
        <div key={option.value} className='flex items-center gap-2'>
          <RadioGroupItem id={`${id}-${option.value}`} value={option.value} />
          <Label htmlFor={`${id}-${option.value}`} className='font-normal'>
            {option.label}
          </Label>
        </div>
      ))}
    </RadioGroup>
  )
}
