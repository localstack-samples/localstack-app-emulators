import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Unmount what a test rendered, so the next one starts on an empty page.
afterEach(cleanup)
