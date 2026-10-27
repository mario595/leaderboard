import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App'

// A first test, to prove the toolchain: Vitest runs, jsdom renders React, and
// the jest-dom matchers (toBeInTheDocument) are loaded by the setup file.
describe('App', () => {
  it('renders the app name', () => {
    render(<App />)
    expect(
      screen.getByRole('heading', { name: 'Leaderboard' }),
    ).toBeInTheDocument()
  })
})
