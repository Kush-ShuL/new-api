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
import { useEffect, type CSSProperties } from 'react'

import { DEFAULT_APPEARANCE } from '@/lib/appearance'
import { useSystemConfigStore } from '@/stores/system-config-store'

export function AppearanceBackground() {
  const appearance = useSystemConfigStore(
    (state) => state.config.appearance ?? DEFAULT_APPEARANCE
  )
  const {
    glassEnabled,
    background,
    imageOpacity,
    maskOpacity,
    surfaceOpacity,
    blurStrength,
  } = appearance

  useEffect(() => {
    document.documentElement.toggleAttribute('data-glass-ui', glassEnabled)
    document.documentElement.toggleAttribute(
      'data-appearance-background',
      glassEnabled && !!background
    )
    document.documentElement.style.setProperty(
      '--glass-opacity',
      `${surfaceOpacity}%`
    )
    document.documentElement.style.setProperty(
      '--glass-blur',
      `${blurStrength}px`
    )
    return () => {
      document.documentElement.removeAttribute('data-glass-ui')
      document.documentElement.removeAttribute('data-appearance-background')
      document.documentElement.style.removeProperty('--glass-opacity')
      document.documentElement.style.removeProperty('--glass-blur')
    }
  }, [glassEnabled, background, surfaceOpacity, blurStrength])

  if (!glassEnabled || !background) return null
  return (
    <div
      className='appearance-background'
      aria-hidden='true'
      style={
        {
          '--image-opacity': imageOpacity / 100,
          '--mask-opacity': maskOpacity / 100,
        } as CSSProperties
      }
    >
      <img
        src={background}
        alt=''
        className='absolute inset-0 h-full w-full object-cover opacity-(--image-opacity)'
      />
      <div className='appearance-mask absolute inset-0 opacity-(--mask-opacity)' />
    </div>
  )
}
