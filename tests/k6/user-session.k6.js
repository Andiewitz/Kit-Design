import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    session: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 1),
      duration: __ENV.DURATION || '60s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
    checks: ['rate>0.99'],
  },
};

const baseURL = __ENV.BASE_URL || 'http://localhost:3001';
const origin = __ENV.APP_ORIGIN || 'http://localhost:5173';
const email = __ENV.USER_EMAIL;
const password = __ENV.USER_PASSWORD;

if (!email || !password) {
  throw new Error('USER_EMAIL and USER_PASSWORD are required; use a pre-verified test account');
}

export default function () {
  http.cookieJar();

  let response = http.post(
    `${baseURL}/v1/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { 'Content-Type': 'application/json', Origin: origin } },
  );
  check(response, {
    'login accepted': (result) => result.status === 200,
    'login returned access token': (result) => Boolean(result.json('access_token')),
  });

  const accessToken = response.json('access_token');
  if (!accessToken) {
    return;
  }

  response = http.get(`${baseURL}/v1/account`, {
    headers: { Authorization: `Bearer ${accessToken}`, Origin: origin },
  });
  check(response, { 'account readable': (result) => result.status === 200 });

  response = http.post(`${baseURL}/v1/auth/refresh`, null, {
    headers: { Origin: origin },
  });
  check(response, { 'refresh accepted': (result) => result.status === 200 });

  const refreshedToken = response.json('access_token');
  if (refreshedToken) {
    response = http.get(`${baseURL}/v1/account`, {
      headers: { Authorization: `Bearer ${refreshedToken}`, Origin: origin },
    });
    check(response, { 'refreshed token readable': (result) => result.status === 200 });
  }

  response = http.post(`${baseURL}/v1/auth/logout`, null, {
    headers: { Authorization: `Bearer ${refreshedToken || accessToken}`, Origin: origin },
  });
  check(response, { 'logout accepted': (result) => result.status === 204 });

  sleep(7);
}
