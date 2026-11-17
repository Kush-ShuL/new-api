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
import {
  AudioLines,
  File,
  ListChecks,
  Image,
  Type,
  VectorSquare,
  Video,
} from 'lucide-react'

export const MODEL_MODALITY_OPTIONS = [
  { value: 'text', label: 'Text', icon: Type },
  { value: 'image', label: 'Image', icon: Image },
  { value: 'audio', label: 'Audio', icon: AudioLines },
  { value: 'video', label: 'Video', icon: Video },
  { value: 'file', label: 'File', icon: File },
  { value: 'embedding', label: 'Embeddings', icon: VectorSquare },
  { value: 'decision', label: 'Decision', icon: ListChecks },
] as const

export const MODEL_INPUT_MODALITY_OPTIONS = MODEL_MODALITY_OPTIONS.filter(
  (modality) => modality.value !== 'embedding' && modality.value !== 'decision'
)
