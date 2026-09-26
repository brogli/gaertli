// PickerRow is the part of a row PickerList needs (a unique key, whether the
// row only labels a group) plus an action row's optional shortcut hint,
// shared so every picker's actionRow can pass one. Its own module: a type
// exported from a <script setup> block resolves to `any` under eslint's
// type-aware linting.
export interface PickerRow {
  key: string
  header?: boolean
  shortcut?: string
}
