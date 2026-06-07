import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import dotenv from 'dotenv';

// Load .env.${ENV} when present; otherwise rely on env vars already set in the process.
const environment = process.env.ENV;
if (environment) {
  const envFilePath = path.join(path.dirname(fileURLToPath(import.meta.url)), `.env.${environment}`);
  if (existsSync(envFilePath)) {
    console.log(`Loading environment file: ${envFilePath}`);
    dotenv.config({ path: envFilePath });
  }
}
