import http from "k6/http";
import exec from "k6/execution";
import { check } from "k6";
import { Counter } from "k6/metrics";

const total = Number(__ENV.TOTAL_ITERATIONS || "1000000");
const pairs = Number(__ENV.PAIR_COUNT || "100");
const vus = Number(__ENV.VUS || "50");
const adapter = __ENV.ADAPTER;
const expectedStatus = Number(__ENV.EXPECTED_STATUS);
const runID = __ENV.RUN_ID;

const successfulTransfers = new Counter("successful_transfers");
const unexpectedStatuses = new Counter("unexpected_statuses");
const responses200 = new Counter("responses_200");
const responses201 = new Counter("responses_201");
const responses400 = new Counter("responses_400");
const responses404 = new Counter("responses_404");
const responses409 = new Counter("responses_409");
const responses422 = new Counter("responses_422");
const responses500 = new Counter("responses_500");
const responsesOther = new Counter("responses_other");

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ["count", "avg", "med", "p(90)", "p(95)", "p(99)", "min", "max"],
  scenarios: {
    million_transfers: {
      executor: "shared-iterations",
      vus,
      iterations: total,
      maxDuration: __ENV.MAX_DURATION || "24h",
      gracefulStop: "0s",
    },
  },
  thresholds: {
    iterations: [`count==${total}`],
    http_reqs: [`count==${total}`],
    successful_transfers: [`count==${total}`],
    unexpected_statuses: ["count==0"],
    http_req_failed: ["rate==0"],
    checks: ["rate==1"],
  },
};

function suffix(pair) {
  return String(pair).padStart(3, "0");
}

function uuid(kind, pair) {
  const prefix = kind === "source" ? "10000000" : "20000000";
  return `${prefix}-0000-4000-8000-${String(pair).padStart(12, "0")}`;
}

function requestFor(iteration) {
  const pair = iteration % pairs;
  const id = suffix(pair);

  switch (adapter) {
    case "chatgpt-c1":
      return ["/transfers", { source_user_id: `k6-src-${id}`, destination_user_id: `k6-dst-${id}`, amount: "0.01" }];
    case "chatgpt-c2":
      return ["/v1/transfers", { source_wallet_id: `k6-src-${id}`, destination_wallet_id: `k6-dst-${id}`, amount: "0.01" }];
    case "gemini-c2":
      return ["/transfers", { from_account_id: 1000001 + pair, to_account_id: 2000001 + pair, amount_cents: 1 }];
    case "grok-c2":
      return ["/transfers", { from_wallet_id: `k6-src-${id}`, to_wallet_id: `k6-dst-${id}`, amount: 1 }];
    case "kimi-c1":
      return ["/pix/transfer", { origem: `k6-src-${id}`, destino: `k6-dst-${id}`, valor: 0.01 }];
    case "kimi-c2":
      return ["/v1/transfers", {
        from_wallet_id: uuid("source", pair),
        to_wallet_id: uuid("destination", pair),
        amount_cents: 1,
        idempotency_key: `${runID}-${String(iteration).padStart(7, "0")}`,
      }];
    case "sonnet-c2":
      return ["/transfers", { from_wallet_id: `k6-src-${id}`, to_wallet_id: `k6-dst-${id}`, amount_cents: 1 }];
    default:
      throw new Error(`unsupported adapter: ${adapter}`);
  }
}

export default function () {
  const iteration = exec.scenario.iterationInTest;
  const [path, body] = requestFor(iteration);
  const response = http.post(`${__ENV.BASE_URL}${path}`, JSON.stringify(body), {
    headers: {
      "Content-Type": "application/json",
      "X-Request-ID": `${runID}-${iteration}`,
    },
    responseCallback: http.expectedStatuses(expectedStatus),
    timeout: __ENV.REQUEST_TIMEOUT || "30s",
    tags: { name: `POST ${path}`, run_id: runID },
  });

  const accepted = check(response, {
    "expected success status": (result) => result.status === expectedStatus,
  });
  const statusCounters = {
    200: responses200,
    201: responses201,
    400: responses400,
    404: responses404,
    409: responses409,
    422: responses422,
    500: responses500,
  };
  (statusCounters[response.status] || responsesOther).add(1);
  if (accepted) {
    successfulTransfers.add(1);
  } else {
    unexpectedStatuses.add(1);
  }
}
