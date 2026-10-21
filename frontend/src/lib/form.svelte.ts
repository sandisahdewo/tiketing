// Form validation shown inline under each field (see components/Field.svelte).
// Fields are checked when the form is submitted; after that every error
// updates as the user types, so it disappears as soon as the field is fixed.

export type Errors = Record<string, string | undefined>

export class FormState {
  submitted = $state(false)
  // Messages from the server, placed under the field they are about.
  server = $state<Errors>({})
  // A server message that is not about any one field.
  general = $state('')

  #validate: () => Errors

  constructor(validate: () => Errors) {
    this.#validate = validate
  }

  errors = $derived.by((): Errors => ({ ...(this.submitted ? this.#validate() : {}), ...this.server }))

  error(field: string): string {
    return this.errors[field] ?? ''
  }

  // check runs at submit: it shows every error and reports whether the form
  // may be sent. The first wrong field gets the focus.
  check(): boolean {
    this.submitted = true
    this.server = {}
    this.general = ''
    const ok = !Object.values(this.#validate()).some(Boolean)
    if (!ok) focusFirstError()
    return ok
  }

  // fail shows a server error under the first field whose pattern matches
  // the message, or above the buttons when none does.
  fail(message: string, fields: Record<string, RegExp> = {}) {
    const field = Object.keys(fields).find((f) => fields[f].test(message))
    if (field) {
      this.server = { [field]: message }
      focusFirstError()
    } else {
      this.general = message
    }
  }

  // clear drops a server error once its field is edited.
  clear(field: string) {
    if (this.server[field]) this.server = { ...this.server, [field]: undefined }
  }

  reset() {
    this.submitted = false
    this.server = {}
    this.general = ''
  }
}

export function focusFirstError() {
  requestAnimationFrame(() => {
    document
      .querySelector<HTMLElement>('.field-invalid input:not([type=hidden]), .field-invalid select, .field-invalid textarea, .field-invalid button')
      ?.focus()
  })
}

// Rules shared by the forms; each returns a message or undefined.

export function required(value: string | number | null | undefined, message: string): string | undefined {
  if (value == null || (typeof value === 'string' && value.trim() === '')) return message
}

export function maxLength(value: string, max: number, message: string): string | undefined {
  if ([...value.trim()].length > max) return message
}

// Same rule as the server: lower-cased, 3-32 of a-z 0-9 . _ -
export function username(value: string): string | undefined {
  const v = value.trim().toLowerCase()
  if (v === '') return 'Username wajib diisi'
  if (!/^[a-z0-9._-]{3,32}$/.test(v)) return 'Username 3-32 karakter: huruf kecil, angka, titik, minus, atau garis bawah'
}
