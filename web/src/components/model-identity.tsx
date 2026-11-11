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
import { useModelCustomizations } from '@/hooks/use-model-customizations'
import { getLobeIcon } from '@/lib/lobe-icon'

export function ModelIdentity(props: {
  name: string
  icon?: string
  size?: number
  showId?: boolean
  showIcon?: boolean
}) {
  const custom = useModelCustomizations()[props.name]
  return (
    <span
      className='inline-flex max-w-full min-w-0 items-center gap-2'
      title={props.name}
    >
      {props.showIcon !== false && (custom?.icon || props.icon) && (
        <span className='shrink-0' aria-hidden='true'>
          {getLobeIcon(custom?.icon || props.icon, props.size ?? 24)}
        </span>
      )}
      <span className='min-w-0'>
        <span className='block truncate'>
          {custom?.display_name || props.name}
        </span>
        {props.showId && custom?.display_name && (
          <span className='text-muted-foreground block truncate text-xs'>
            {props.name}
          </span>
        )}
      </span>
    </span>
  )
}
