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
import { useState, type ReactNode } from 'react'

import { useSystemConfigStore } from '@/stores/system-config-store'

export function CurrencyIcon(props: { fallback?: boolean }): ReactNode {
  const currency = useSystemConfigStore((state) => state.config.currency)
  const [failedUrl, setFailedUrl] = useState('')
  const url = currency.customCurrencyIcon ?? ''
  if (
    currency.quotaDisplayType !== 'CUSTOM' ||
    (!props.fallback && !currency.displayInCurrency)
  ) {
    return null
  }
  if (!url || url === failedUrl) {
    return props.fallback ? `${currency.customCurrencySymbol} ` : null
  }
  return (
    <img
      src={url}
      alt={currency.customCurrencySymbol}
      className='me-[0.25em] inline-block size-[1em] shrink-0 object-contain align-[-0.125em]'
      onError={() => setFailedUrl(url)}
    />
  )
}

// Keep numeric precision and plain-text formatting in the currency library.
// Only replace a formatted custom-currency prefix at the point of rendering.
export function CurrencyText(props: { children: ReactNode }): ReactNode {
  const currency = useSystemConfigStore((state) => state.config.currency)
  const symbol = currency.customCurrencySymbol
  const text = props.children
  if (
    currency.quotaDisplayType !== 'CUSTOM' ||
    !currency.customCurrencyIcon ||
    !symbol ||
    typeof text !== 'string' ||
    !text.startsWith(symbol) ||
    (text !== symbol &&
      !text.startsWith(`${symbol} `) &&
      !/^[-+]?\d/.test(text.slice(symbol.length)))
  ) {
    return props.children
  }
  return (
    <>
      <CurrencyIcon fallback />
      {text.slice(symbol.length).trimStart()}
    </>
  )
}
