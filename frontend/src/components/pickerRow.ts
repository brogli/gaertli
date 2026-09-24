// PickerRow is the part of a row PickerList needs: a unique key, and
// whether the row only labels a group. Its own module: a type exported from
// a <script setup> block resolves to `any` under eslint's type-aware linting.
export interface PickerRow {
  key: string
  header?: boolean
}
