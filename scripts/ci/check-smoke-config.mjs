import { readFileSync } from 'node:fs';

const smoke = readFileSync(new URL('./smoke-e2e.sh', import.meta.url), 'utf8');

const requirements = [
  [
    'the integration smoke must exercise authoritative final inventory deduction',
    /export INVENTORY_FINAL_DEDUCT_ENABLED="true"/,
  ],
  [
    'the ephemeral auth config must expose the verification code used by registration',
    /s\/ExposeDebugCode: false\/ExposeDebugCode: true\//,
  ],
  [
    'the auth service must start from the ephemeral smoke config',
    /start_go_service "auth-api" "\.\/app\/auth\/api\/auth\.go" "\$\{auth_api_smoke_config\}"/,
  ],
];

for (const [description, pattern] of requirements) {
  if (!pattern.test(smoke)) {
    throw new Error(`smoke configuration check failed: ${description}`);
  }
}

console.log('integration smoke configuration verified');
