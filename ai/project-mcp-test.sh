#!/usr/bin/env bash
# Isolated filesystem acceptance checks; no MCP processes or package installs.
set -uo pipefail
here="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
command -v node >/dev/null || { echo 'node is required to run this suite' >&2; exit 2; }
node_bin="$(command -v node)"
tmp="$(mktemp -d)" || exit 2
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/home" "$tmp/project é"
cat >"$tmp/bin/mise" <<'FAKE'
#!/bin/sh
[ "$1 $2 $3" = 'exec node@lts --' ] || exit 99
shift 3
exec "$NODE_BIN" "$@"
FAKE
chmod +x "$tmp/bin/mise"
export NODE_BIN="$node_bin" HOME="$tmp/home" PATH="$tmp/bin:$PATH"
export PROJECT_MCP_SCRIPT="$here/project-mcp.sh" PROJECT_MCP_TMP="$tmp"
node <<'JS'
const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const assert = require('node:assert/strict');
const root = process.env.PROJECT_MCP_TMP;
let passed = 0, failed = 0;
function test(name, action) {
  try { action(); passed++; } catch (error) { failed++; console.error(`FAIL ${name}: ${error.message}`); }
}
function run(agent, project, ...args) {
  return spawnSync('bash', [process.env.PROJECT_MCP_SCRIPT, `--agent=${agent}`, ...args, project], {encoding:'utf8'});
}
function read(file) { return fs.readFileSync(file, 'utf8'); }
for (const agent of ['claude', 'codex']) {
  const project = path.join(root, `project é ${agent}`);
  fs.mkdirSync(project);
  const file = path.join(project, agent === 'claude' ? '.mcp.json' : '.codex/config.toml');
  test(`${agent} dry-run writes nothing`, () => {
    assert.equal(run(agent, project, '--dry-run').status, 0);
    assert.deepEqual(fs.readdirSync(project), []);
  });
  test(`${agent} install exports both public browsers portably`, () => {
    assert.equal(run(agent, project).status, 0);
    const text = read(file);
    assert.ok(text.includes('playwright'));
    assert.ok(text.includes('chrome-devtools'));
    assert.ok(text.includes('$HOME/.kk-flavor'));
    assert.ok(text.includes('../mcp-env.sh'));
    assert.ok(!text.includes(root));
  });
  test(`${agent} reinstall is byte-idempotent`, () => {
    const before = read(file);
    assert.equal(run(agent, project).status, 0);
    assert.equal(read(file), before);
  });
  test(`${agent} uninstall preserves unrelated settings and server`, () => {
    if (agent === 'claude') {
      const value = JSON.parse(read(file));
      value.custom = 'keep'; value.mcpServers.other = {command:'keep'};
      fs.writeFileSync(file, JSON.stringify(value));
    } else { fs.appendFileSync(file, '\n[mcp_servers.other]\ncommand = "keep"\n'); }
    assert.equal(run(agent, project, '--uninstall').status, 0);
    const text = read(file);
    assert.ok(text.includes('keep'));
    assert.ok(!text.includes('@playwright/mcp'));
    assert.equal(run(agent, project).status, 0);
    assert.ok(read(file).includes('keep'));
  });
  test(`${agent} conflict is refused without changing config`, () => {
    const text = agent === 'claude' ? '{"mcpServers":{"playwright":{"command":"mine"}}}' : '[mcp_servers.playwright]\ncommand = "mine"\n';
    fs.writeFileSync(file, text);
    assert.notEqual(run(agent, project).status, 0);
    assert.equal(read(file), text);
  });
  test(`${agent} config symlink cannot escape project`, () => {
    fs.unlinkSync(file);
    const outside = path.join(root, `${agent}-outside`);
    fs.writeFileSync(outside, '{}'); fs.symlinkSync(outside, file);
    assert.notEqual(run(agent, project).status, 0);
    assert.equal(read(outside), '{}');
  });
}
test('portable launcher handles spaces and shell metacharacters in HOME', () => {
  const home = path.join(root, 'home $value \"quote é');
  const checkout = path.join(home, 'checkout');
  fs.mkdirSync(path.join(checkout, 'kk-flavor'), {recursive:true});
  fs.symlinkSync(path.join(checkout, 'kk-flavor'), path.join(home, '.kk-flavor'));
  fs.writeFileSync(path.join(checkout, 'mcp-env.sh'), '#!/bin/sh\nprintf \"%s\n\" \"$@\"\n', {mode:0o755});
  const project = path.join(root, 'launcher'); fs.mkdirSync(project);
  assert.equal(run('claude', project).status, 0);
  const server = JSON.parse(read(path.join(project, '.mcp.json'))).mcpServers.playwright;
  const result = spawnSync(server.command, server.args, {encoding:'utf8', env:{...process.env, HOME:home}});
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, 'mise\nexec\nnode@lts\n--\nnpx\n-y\n@playwright/mcp\n');
});

test('edited managed Codex region is kept on uninstall', () => {
  const project = path.join(root, 'edited'); fs.mkdirSync(project);
  assert.equal(run('codex', project).status, 0);
  const file = path.join(project, '.codex/config.toml');
  const edited = read(file).replace('command = "sh"', 'command = "mine"');
  fs.writeFileSync(file, edited);
  assert.notEqual(run('codex', project, '--uninstall').status, 0);
  assert.equal(read(file), edited);
});

test('Codex refuses a directory symlink', () => {
  const project = path.join(root, 'directory-link'); fs.mkdirSync(project);
  const outside = path.join(root, 'outside-directory'); fs.mkdirSync(outside);
  fs.symlinkSync(outside, path.join(project, '.codex'));
  assert.notEqual(run('codex', project).status, 0);
  assert.deepEqual(fs.readdirSync(outside), []);
});

test('cold dry-run and uninstall never provision Node or evaluate mise config', () => {
  const bin = path.join(root, 'cold-bin'); fs.mkdirSync(bin);
  fs.symlinkSync('/usr/bin/dirname', path.join(bin, 'dirname'));
  const log = path.join(root, 'cold-mise-log');
  fs.writeFileSync(path.join(bin, 'mise'), '#!/bin/sh\nprintf "%s:%s:%s:%s\\n" "$MISE_NO_CONFIG" "$MISE_NO_ENV" "$MISE_NO_HOOKS" "$*" >> "$COLD_MISE_LOG"\nexit 1\n', {mode:0o755});
  const project = path.join(root, 'cold-project'); fs.mkdirSync(project);
  const invoke = (...args) => spawnSync('/bin/bash', [process.env.PROJECT_MCP_SCRIPT, ...args, project], {
    encoding:'utf8', env:{...process.env, PATH:bin, COLD_MISE_LOG:log}
  });
  const preview = invoke('--agent=codex', '--dry-run');
  assert.equal(preview.status, 0, preview.stderr);
  assert.ok(preview.stdout.includes('validation deferred'));
  assert.notEqual(invoke('--agent=codex', '--uninstall').status, 0);
  assert.equal(invoke('--agent=invalid', '--dry-run').status, 2);
  assert.equal(read(log), '1:1:1:where node@lts\n1:1:1:where node@lts\n');
  assert.deepEqual(fs.readdirSync(project), []);
});

test('private MCP definitions are never read or copied', () => {
  const checkout = path.join(root, 'private-fixture'); fs.mkdirSync(checkout);
  const source = path.dirname(process.env.PROJECT_MCP_SCRIPT);
  for (const file of ['project-mcp.sh', 'project-mcp.mjs', 'mcp.jsonc']) fs.copyFileSync(path.join(source, file), path.join(checkout, file));
  fs.writeFileSync(path.join(checkout, 'mcp.private.jsonc'), 'INVALID PRIVATE SECRET');
  const project = path.join(root, 'public-only'); fs.mkdirSync(project);
  for (const agent of ['claude', 'codex']) {
    const result = spawnSync('bash', [path.join(checkout, 'project-mcp.sh'), `--agent=${agent}`, project], {encoding:'utf8'});
    assert.equal(result.status, 0, result.stderr);
    assert.ok(!read(path.join(project, agent === 'claude' ? '.mcp.json' : '.codex/config.toml')).includes('SECRET'));
  }
});

test('HOME cannot be mistaken for a project', () => {
  assert.notEqual(run('codex', process.env.HOME).status, 0);
});

test('uninstall refuses trailing fields that belong to a managed TOML server', () => {
  const project = path.join(root, 'trailing-field'); fs.mkdirSync(project);
  assert.equal(run('codex', project).status, 0);
  const file = path.join(project, '.codex/config.toml');
  fs.appendFileSync(file, 'startup_timeout_sec = 60\n');
  const before = read(file);
  assert.notEqual(run('codex', project, '--uninstall').status, 0);
  assert.equal(read(file), before);
});

for (const agent of ['claude', 'codex']) {
  test(`${agent} ignored project config is reported without rewriting ignore rules`, () => {
    const project = path.join(root, `ignored-${agent}`); fs.mkdirSync(project);
    assert.equal(spawnSync('git', ['-C', project, 'init', '-q']).status, 0);
    const ignore = '.codex/\n.mcp.json\n';
    fs.writeFileSync(path.join(project, '.gitignore'), ignore);
    const result = run(agent, project);
    assert.notEqual(result.status, 0);
    assert.ok(result.stderr.includes('ignored by Git'), result.stderr);
    assert.equal(read(path.join(project, '.gitignore')), ignore);
    assert.ok(!fs.existsSync(path.join(project, agent === 'claude' ? '.mcp.json' : '.codex/config.toml')));
  });
}

for (const runtime of ['installed', 'provision']) {
  test(`mise ${runtime} Node path runs only isolated runtime commands`, () => {
    const bin = path.join(root, `runtime-${runtime}`); fs.mkdirSync(bin);
    fs.symlinkSync('/usr/bin/dirname', path.join(bin, 'dirname'));
    const nodeRoot = path.join(bin, 'node-root'); fs.mkdirSync(path.join(nodeRoot, 'bin'), {recursive:true});
    fs.symlinkSync(process.execPath, path.join(nodeRoot, 'bin/node'));
    const log = path.join(root, `runtime-${runtime}.log`);
    fs.writeFileSync(path.join(bin, 'mise'), `#!/bin/sh
printf '%s:%s:%s:%s\\n' "$MISE_NO_CONFIG" "$MISE_NO_ENV" "$MISE_NO_HOOKS" "$*" >> "$RUNTIME_LOG"
if [ "$1" = where ]; then
  [ "$RUNTIME_MODE" = installed ] || exit 1
  printf '%s\\n' "$RUNTIME_NODE_ROOT"
elif [ "$1" = exec ]; then
  shift 4
  exec "$NODE_BIN" "$@"
else
  exit 9
fi
`, {mode:0o755});
    const project = path.join(root, `runtime-project-${runtime}`); fs.mkdirSync(project);
    const result = spawnSync('/bin/bash', [process.env.PROJECT_MCP_SCRIPT, '--agent=claude', project], {
      encoding:'utf8', env:{...process.env, PATH:bin, RUNTIME_LOG:log, RUNTIME_MODE:runtime, RUNTIME_NODE_ROOT:nodeRoot}
    });
    assert.equal(result.status, 0, result.stderr);
    assert.ok(read(path.join(project, '.mcp.json')).includes('playwright'));
    const lines = read(log).trim().split('\n');
    assert.equal(lines[0], '1:1:1:where node@lts');
    assert.equal(lines.length, runtime === 'installed' ? 1 : 2);
    if (runtime === 'provision') assert.ok(lines[1].startsWith('1:1:1:exec node@lts -- node '));
  });
}

test('no user client settings created', () => assert.deepEqual(fs.readdirSync(process.env.HOME), []));
console.log(`${passed} passed, ${failed} failed`);
process.exitCode = failed ? 1 : 0;
JS
