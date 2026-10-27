import { relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

// SonarQube scans from the repo root and resolves both reports' paths from it. This package sits one
// level below the root. A package at the root drops repoRoot, onWritePath and projectRoot.
const repoRoot = fileURLToPath(new URL('..', import.meta.url));

export default defineConfig({
  test: {
    include: ['src/**/*.unit.test.{ts,tsx}'],
    reporters: [
      'default',
      [
        'vitest-sonar-reporter',
        { outputFile: 'coverage/test-report.xml', onWritePath: (path: string) => relative(repoRoot, resolve(path)) },
      ],
    ],
    // On only under --coverage, which test:ci passes.
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}'],
      reporter: ['text-summary', ['lcovonly', { projectRoot: repoRoot }]],
      thresholds: { lines: 80, branches: 80, functions: 80, statements: 80 },
    },
  },
});
