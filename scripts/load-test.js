// load-test.js — 서버 부하 테스트 (k6, 7.8장 표준)
// 실행: k6 run scripts/load-test.js
// 임계값: p95 < 300ms, 오류율 < 1%
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

export const options = {
  vus: 50,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<300'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || '';

// /health + /gbridge/call(모델 목록) 왕복 시나리오
export default function () {
  const health = http.get(`${BASE}/api/v1/health`);
  check(health, { 'health 200': (r) => r.status === 200 });

  const headers = TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {};
  const models = http.get(`${BASE}/api/v1/models`, { headers });
  check(models, { 'models 200': (r) => r.status === 200 });

  sleep(0.2);
}