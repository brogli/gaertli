// PickerRow is the part of a row PickerList needs: a unique key, and whether
// the row only labels a group. Its own module: a type exported from a
// <script setup> block resolves to `any` under eslint's type-aware linting.
export interface PickerRow {
  key: string
  header?: boolean
}

// filterRows keeps the rows whose text contains query, ignoring case and
// the query's surrounding blanks; all of them for a blank query.
export function filterRows<Row>(rows: Row[], query: string, text: (row: Row) => string): Row[] {
  const q = query.trim().toLowerCase()
  return q ? rows.filter((r) => text(r).toLowerCase().includes(q)) : rows
}
