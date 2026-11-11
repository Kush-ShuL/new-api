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
export function CurrencyText(props: {
  children: ReactNode
  label?: boolean
  embedded?: boolean
}): ReactNode {
  const currency = useSystemConfigStore((state) => state.config.currency)
  const symbol = currency.customCurrencySymbol
  const text = props.children
  if (
    props.embedded &&
    currency.quotaDisplayType === 'CUSTOM' &&
    currency.customCurrencyIcon &&
    symbol &&
    typeof text === 'string'
  ) {
    const amounts = new RegExp(
      `${symbol.replaceAll(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*[-+]?\\d[\\d.,]*`,
      'g'
    )
    const parts: ReactNode[] = []
    let offset = 0
    for (const match of text.matchAll(amounts)) {
      parts.push(
        text.slice(offset, match.index),
        <CurrencyText key={match.index}>{match[0]}</CurrencyText>
      )
      offset = match.index + match[0].length
    }
    parts.push(text.slice(offset))
    return parts
  }
  if (
    props.label &&
    currency.quotaDisplayType === 'CUSTOM' &&
    currency.customCurrencyIcon &&
    symbol &&
    typeof text === 'string' &&
    text.endsWith(`(${symbol})`)
  ) {
    return (
      <span className='inline-flex items-baseline whitespace-nowrap'>
        {text.slice(0, -symbol.length - 1)}
        <CurrencyIcon fallback />)
      </span>
    )
  }
  if (
    currency.quotaDisplayType !== 'CUSTOM' ||
    !currency.customCurrencyIcon ||
    !symbol ||
    typeof text !== 'string' ||
    !text.startsWith(symbol) ||
    (text !== symbol &&
      !/^\s*[-+]?\d/.test(text.slice(symbol.length)) &&
      !/^\s*\//.test(text.slice(symbol.length)))
  ) {
    return props.children
  }
  return (
    <span className='inline-flex max-w-full items-baseline align-baseline whitespace-nowrap'>
      <CurrencyIcon fallback />
      <span
        className='min-w-0 truncate'
        title={text.slice(symbol.length).trimStart()}
      >
        {text.slice(symbol.length).trimStart()}
      </span>
    </span>
  )
}
