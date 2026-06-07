import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import dotenv from 'dotenv';

// Default to development so a bare local run loads .env.development. A deploy sets
// ENV explicitly and ships no .env file, so a missing file just no-ops here.
const environment = process.env.ENV ?? 'development';
const envFilePath = path.join(path.dirname(fileURLToPath(import.meta.url)), `.env.${environment}`);
if (existsSync(envFilePath)) {
  dotenv.config({ path: envFilePath, quiet: true });
}
