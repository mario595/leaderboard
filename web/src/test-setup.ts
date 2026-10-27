import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
import '@testing-library/jest-dom/vitest'

// Testing Library unmounts what each test rendered only when the runner
// exposes afterEach as a global. Vitest does not by default (globals: false),
// so register the cleanup explicitly: otherwise every test sees the pages
// rendered by the tests before it.
afterEach(() => {
  cleanup()
})
