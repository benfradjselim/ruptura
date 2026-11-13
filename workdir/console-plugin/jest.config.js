// Unit tests cover the pure modules under src/util; the React components are
// exercised in the console on the lab cluster.
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  roots: ['<rootDir>/src'],
  testMatch: ['**/*.test.ts'],
};
