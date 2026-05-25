// frontend/jest.config.js
// Drop this into the frontend/ directory.
// Works with Next.js 13+ App Router + TypeScript + React Testing Library.

const nextJest = require('next/jest')

const createJestConfig = nextJest({
  // Path to your Next.js app (relative to this file)
  dir: './',
})

/** @type {import('jest').Config} */
const customConfig = {
  // Use jsdom so React components can render
  testEnvironment: 'jsdom',

  // Run this setup file before each test suite to configure
  // @testing-library/jest-dom matchers (toBeInTheDocument etc.)
  setupFilesAfterFramework: ['<rootDir>/jest.setup.ts'],

  // Collect coverage from exactly the files SonarQube needs to see
  collectCoverageFrom: [
    'lib/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
    'app/**/*.{ts,tsx}',
    // Exclude test files themselves and type-only files
    '!**/*.test.{ts,tsx}',
    '!**/*.spec.{ts,tsx}',
    '!**/*.d.ts',
    '!**/node_modules/**',
  ],

  // lcov   → SonarQube reads frontend/coverage/lcov.info
  // text   → prints a summary table to the console
  // json   → used by some CI dashboards
  coverageReporters: ['lcov', 'text', 'json-summary'],

  // Where to write coverage output (must match Jenkinsfile + sonar-project.properties)
  coverageDirectory: 'coverage',

  // Optionally set a minimum threshold to make the pipeline fail early
  // before even reaching the SonarQube quality gate.
  // coverageThreshold: {
  //   global: {
  //     branches: 60,
  //     functions: 60,
  //     lines: 60,
  //     statements: 60,
  //   },
  // },

  // Module name mapper for absolute imports (e.g. "@/components/...")
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },

  // Transform TypeScript and TSX via the Next.js Jest transformer
  // (next/jest already sets this up, but listed here for clarity)
  // transform is inherited from createJestConfig

  testMatch: [
    '**/__tests__/**/*.[jt]s?(x)',
    '**/?(*.)+(spec|test).[jt]s?(x)',
  ],

  // Ignore build output
  testPathIgnorePatterns: [
    '<rootDir>/node_modules/',
    '<rootDir>/.next/',
  ],
}

// createJestConfig merges Next.js defaults (SWC transform, CSS mocking, etc.)
module.exports = createJestConfig(customConfig)
