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
import { useSystemConfigStore } from '@/stores/system-config-store'

// VChart builds its tooltip outside React, so replace only formatted monetary
// values in its value column. Model names and other tooltip text stay untouched.
export const CURRENCY_TOOLTIP = {
  className: 'newapi-chart-tooltip',
  updateElement(element: HTMLElement) {
    const currency = useSystemConfigStore.getState().config.currency
    const symbol = currency.customCurrencySymbol
    const url = currency.customCurrencyIcon
    if (currency.quotaDisplayType !== 'CUSTOM' || !symbol || !url) return
    for (const row of element.querySelectorAll<HTMLElement>(
      '[data-col="value"] > div'
    )) {
      const text = row.textContent ?? ''
      if (
        !text.startsWith(symbol) ||
        !/^\s*[-+]?\d/.test(text.slice(symbol.length))
      ) {
        continue
      }
      const amount = document.createElement('span')
      amount.className = 'inline-flex items-baseline whitespace-nowrap'
      const icon = document.createElement('img')
      icon.src = url
      icon.alt = symbol
      icon.className =
        'me-[0.25em] inline-block size-[1em] shrink-0 object-contain'
      icon.addEventListener(
        'error',
        () => {
          amount.textContent = text
        },
        { once: true }
      )
      amount.append(icon, text.slice(symbol.length).trimStart())
      row.replaceChildren(amount)
    }
  },
}

export const VCHART_OPTION = {
  // 与老前端保持一致（浏览器环境渲染优化）
  mode: 'desktop-browser',
} as const
