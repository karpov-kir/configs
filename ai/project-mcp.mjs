// Project-only MCP configuration. Tested by project-mcp-test.sh.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { isDeepStrictEqual } from 'node:util';
import { spawnSync } from 'node:child_process';

const here = path.dirname(fileURLToPath(import.meta.url));
const launcher = 'bucket=$(CDPATH= cd -P "$HOME/.kk-flavor" && pwd -P) || exit; exec "$bucket/../mcp-env.sh" "$@"';
const open = '# kk-flavor-mcp:begin';
const close = '# kk-flavor-mcp:end';

function parseArguments() {
  let agent, project;
  let isDryRun = false, isUninstall = false;
  for (const arg of process.argv.slice(2)) {
    if (arg === '--agent=claude' || arg === '--agent=codex') agent = arg.slice(8);
    else if (arg === '--dry-run') isDryRun = true;
    else if (arg === '--uninstall') isUninstall = true;
    else if (arg.startsWith('-') || project) throw new Error(`unknown or extra argument: ${arg}`);
    else project = arg;
  }
  if (!agent || !project) throw new Error('--agent=claude|codex and a project directory are required');
  project = fs.realpathSync(project);
  if (process.env.HOME && project === fs.realpathSync(process.env.HOME)) throw new Error('the home directory is not a project target');
  if (!fs.statSync(project).isDirectory()) throw new Error('project must be a directory');
  return {agent, project, isDryRun, isUninstall};
}

function readPublicServers() {
  const source = fs.readFileSync(path.join(here, 'mcp.jsonc'), 'utf8').replace(/^\s*\/\/.*$/gm, '');
  const servers = JSON.parse(source).mcpServers;
  if (!servers || Array.isArray(servers) || typeof servers !== 'object') throw new Error('public mcpServers must be an object');
  return Object.fromEntries(Object.entries(servers).map(([name, server]) => {
    if (!/^[a-zA-Z0-9_-]+$/.test(name) || server.type !== 'stdio' || server.command !== '@CONFIGS@/mcp-env.sh' ||
        !Array.isArray(server.args) || !server.args.every(arg => typeof arg === 'string' && !arg.includes('\0')) ||
        Object.keys(server).some(key => !['type', 'command', 'args'].includes(key))) {
      throw new Error(`public server ${name} needs an explicit portable project mapping`);
    }
    return [name, {type:'stdio', command:'sh', args:['-c', launcher, 'kk-flavor-mcp', ...server.args]}];
  }));
}

function readConfig(file) {
  try {
    const stat = fs.lstatSync(file);
    if (!stat.isFile() || stat.nlink !== 1) throw new Error(`${file} must be a regular unlinked file`);
    return fs.readFileSync(file, 'utf8');
  } catch (error) {
    if (error.code === 'ENOENT') return '';
    throw error;
  }
}

function mergeClaude({text, servers, isUninstall}) {
  const config = text ? JSON.parse(text) : {};
  if (!config || typeof config !== 'object' || Array.isArray(config)) throw new Error('.mcp.json must be an object');
  const current = config.mcpServers ?? {};
  if (!current || typeof current !== 'object' || Array.isArray(current)) throw new Error('mcpServers must be an object');
  let hasChanged = false;
  for (const [name, server] of Object.entries(servers)) {
    if (Object.hasOwn(current, name)) {
      if (!isDeepStrictEqual(current[name], server)) {
        if (isUninstall) continue;
        throw new Error(`project MCP server ${name} already exists with different settings`);
      }
      if (isUninstall) { delete current[name]; hasChanged = true; }
    } else if (!isUninstall) { current[name] = server; hasChanged = true; }
  }
  if (!hasChanged) return text;
  config.mcpServers = current;
  return JSON.stringify(config, undefined, 2) + '\n';
}

function mergeCodex({text, servers, isUninstall}) {
  const region = `${open}\n` + Object.entries(servers).map(([name, server]) =>
    `[mcp_servers.${name}]\ncommand = "sh"\nargs = ${JSON.stringify(server.args)}\n`).join('\n') + `${close}\n`;
  if (text.includes("'''") || text.includes('\"\"\"')) throw new Error('multiline TOML strings require an explicit MCP merge');
  let outside = text;
  const start = text.indexOf(open), end = text.indexOf(close);
  if (start >= 0 || end >= 0) {
    if (start < 0 || end < start || text.indexOf(open, start + open.length) >= 0 || text.indexOf(close, end + close.length) >= 0) {
      throw new Error('incomplete or repeated project MCP markers');
    }
    if ((start > 0 && text[start - 1] !== '\n') || (end > 0 && text[end - 1] !== '\n') ||
        !['', '\n'].includes(text.slice(end + close.length, end + close.length + 1))) {
      throw new Error('project MCP markers must occupy whole lines');
    }
    const owned = text.slice(start, end + close.length) + '\n';
    if (owned !== region) throw new Error('project MCP region was edited; preserve or remove it explicitly');
    const suffix = text.slice(end + close.length);
    const firstFollowingSetting = suffix.split('\n').find(line => line.trim() && !/^\s*#/.test(line));
    if (firstFollowingSetting && !/^\s*\[/.test(firstFollowingSetting)) {
      throw new Error('settings after the MCP region belong to a managed server; move them before removing it');
    }
    outside = text.slice(0, start) + text.slice(end + close.length).replace(/^\n/, '');
  }
  if (isUninstall) return outside;
  // Recognize ordinary server tables only. Other spellings could redefine the parent table or a
  // managed server, so leave those files for an explicit edit instead of guessing at TOML syntax.
  for (const line of outside.split('\n')) {
    if (/^\s*#/.test(line)) continue;
    if (/\\[uU]/.test(line)) throw new Error('escaped TOML requires an explicit MCP merge');
    if (!line.includes('mcp_servers')) continue;
    const match = line.match(/^\s*\[mcp_servers\.([a-zA-Z0-9_-]+)(?:\.[a-zA-Z0-9_-]+)*\]\s*(?:#.*)?$/);
    if (!match || Object.hasOwn(servers, match[1])) throw new Error('existing MCP TOML conflicts or uses ambiguous syntax; merge it explicitly');
  }
  if (start >= 0) return text;
  return outside + (outside && !outside.endsWith('\n') ? '\n' : '') + region;
}

function writeConfig({file, text, previous}) {
  if (text === previous) return;
  fs.mkdirSync(path.dirname(file), {recursive:true});
  const temporary = `${file}.kk-mcp-${process.pid}`;
  let wasCreated = false;
  try {
    const mode = fs.existsSync(file) ? fs.statSync(file).mode : 0o644;
    fs.writeFileSync(temporary, text, {flag:'wx', mode});
    wasCreated = true;
    fs.renameSync(temporary, file);
  } finally {
    if (wasCreated && fs.existsSync(temporary)) fs.unlinkSync(temporary);
  }
}

try {
  const options = parseArguments();
  const directory = path.join(options.project, '.codex');
  if (options.agent === 'codex') {
    try {
      if (!fs.lstatSync(directory).isDirectory()) throw new Error('.codex must be a real directory, not a link');
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
  }
  const file = path.join(options.project, options.agent === 'claude' ? '.mcp.json' : '.codex/config.toml');
  if (!options.isUninstall) {
    const relative = path.relative(options.project, file);
    const git = (...args) => spawnSync('git', ['-C', options.project, ...args], {encoding:'utf8'});
    if (git('rev-parse', '--is-inside-work-tree').status === 0) {
      const ignored = git('check-ignore', '--quiet', '--', relative);
      if (ignored.status === 0) {
        throw new Error(`${relative} is ignored by Git; adjust the project ignore rules to include this configuration, then rerun and review it before committing`);
      }
      if (ignored.status !== 1) throw new Error(`cannot check whether Git will track ${relative}`);
    }
  }
  const previous = readConfig(file);
  const input = {...options, text:previous, servers:readPublicServers()};
  const text = options.agent === 'claude' ? mergeClaude(input) : mergeCodex(input);
  if (!options.isDryRun) writeConfig({file, text, previous});
  console.log(`project MCP: ${options.isDryRun ? 'would update' : 'checked'} ${file}`);
} catch (error) {
  console.error(`project MCP: ${error.message}`);
  process.exitCode = 1;
}
