// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import Long from "long";

import * as protos from "@cockroachlabs/crdb-protobuf-client";
import {
  aggregateNumericStats,
  NumericStat,
  flattenStatementStats,
  StatementStatistics,
  ExecStats,
  combineStatementStats,
} from "./appStats";
import IExplainTreePlanNode = protos.cockroach.sql.IExplainTreePlanNode;
import ISensitiveInfo = protos.cockroach.sql.ISensitiveInfo;
import { random } from "d3";
import { exec } from "child_process";

// record is implemented here so we can write the below test as a direct
// analog of the one in pkg/roachpb/app_stats_test.go.  It's here rather
// than in the main source file because we don't actually need it for the
// application to use.
function record(l: NumericStat, count: number, val: number) {
  const delta = val - l.mean;
  l.mean += delta / count;
  l.squared_diffs += delta * (val - l.mean);
}

function emptyStats() {
  return {
    mean: 0,
    squared_diffs: 0,
  };
}

function makeSensitiveInfo(
  lastErr: string,
  planDescription: IExplainTreePlanNode,
): ISensitiveInfo {
  return {
    last_err: lastErr,
    most_recent_plan_description: planDescription,
  };
}

describe("addNumericStats", () => {
  it("adds two numeric stats together", () => {
    const aData = [1.1, 3.3, 2.2];
    const bData = [2.0, 3.0, 5.5, 1.2];

    let countA = 0;
    let countB = 0;
    let countAB = 0;

    let sumA = 0;
    let sumB = 0;
    let sumAB = 0;

    const a = emptyStats();
    const b = emptyStats();
    const ab = emptyStats();

    aData.forEach(v => {
      countA++;
      sumA += v;
      record(a, countA, v);
    });

    bData.forEach(v => {
      countB++;
      sumB += v;
      record(b, countB, v);
    });

    bData.concat(aData).forEach(v => {
      countAB++;
      sumAB += v;
      record(ab, countAB, v);
    });

    expect(Math.abs(a.mean - 2.2)).toBeLessThanOrEqual(0.0000001);
    expect(Math.abs(a.mean - sumA / countA)).toBeLessThanOrEqual(0.0000001);
    expect(Math.abs(b.mean - sumB / countB)).toBeLessThanOrEqual(0.0000001);
    expect(Math.abs(ab.mean - sumAB / countAB)).toBeLessThanOrEqual(0.0000001);

    const combined = aggregateNumericStats(a, b, countA, countB);

    expect(Math.abs(combined.mean - ab.mean)).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(combined.squared_diffs - ab.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    const reversed = aggregateNumericStats(b, a, countB, countA);

    expect(reversed).toEqual(combined);
  });
});

describe("flattenStatementStats", () => {
  it("flattens CollectedStatementStatistics to ExecutionStatistics", () => {
    const stats = [
      {
        key: {
          key_data: {
            query: "SELECT * FROM foobar",
            query_summary: "SELECT * FROM foobar",
            app: "foobar",
            distSQL: true,
            vec: false,
            opt: true,
            full_scan: true,
            failed: false,
          },
          node_id: 1,
        },
        stats: {},
      },
      {
        key: {
          key_data: {
            query: "UPDATE foobar SET name = 'baz' WHERE id = 42",
            query_summary: "UPDATE foobar SET name = 'baz' WHERE id = 42",
            app: "bazzer",
            distSQL: false,
            vec: false,
            opt: false,
            full_scan: false,
            failed: true,
          },
          node_id: 2,
        },
        stats: {},
      },
      {
        key: {
          key_data: {
            query:
              "SELECT app_name, aggregated_ts, fingerprint_id, metadata, statistics FROM system.app_statistics JOIN system.transaction_statistics ON crdb_internal.transaction_statistics.app_name = system.transaction_statistics.app_name",
            query_summary:
              "SELECT app_name, aggre... FROM system.app_statistics JOIN sys...",
            app: "unique_pear",
            distSQL: false,
            vec: false,
            opt: false,
            full_scan: true,
            failed: true,
          },
          node_id: 3,
        },
        stats: {},
      },
      {
        key: {
          key_data: {
            query:
              "INSERT INTO system.public.lease(\"descID\", version, \"nodeID\", expiration) VALUES ('1232', '111', __more2__)",
            query_summary:
              'INSERT INTO system.public.lease("descID", versi...)',
            app: "test_summary",
            distSQL: false,
            vec: false,
            opt: false,
            full_scan: false,
            failed: true,
          },
          node_id: 4,
        },
        stats: {},
      },
      {
        key: {
          key_data: {
            query:
              "UPDATE system.jobs SET status = $2, payload = $3, last_run = $4, num_runs = $5 WHERE internal_table_id = $1",
            query_summary:
              "UPDATE system.jobs SET status = $2, pa... WHERE internal_table_...",
            app: "test1",
            distSQL: false,
            vec: false,
            opt: false,
            full_scan: false,
            failed: true,
          },
          node_id: 5,
        },
        stats: {},
      },
    ];

    const flattened = flattenStatementStats(stats);

    expect(flattened.length).toEqual(stats.length);

    for (let i = 0; i < flattened.length; i++) {
      expect(flattened[i].statement).toEqual(stats[i].key.key_data.query);
      expect(flattened[i].statement_summary).toEqual(
        stats[i].key.key_data.query_summary,
      );
      expect(flattened[i].app).toEqual(stats[i].key.key_data.app);
      expect(flattened[i].distSQL).toEqual(stats[i].key.key_data.distSQL);
      expect(flattened[i].vec).toEqual(stats[i].key.key_data.vec);
      expect(flattened[i].full_scan).toEqual(stats[i].key.key_data.full_scan);
      expect(flattened[i].failed).toEqual(stats[i].key.key_data.failed);
      expect(flattened[i].node_id).toEqual(stats[i].key.node_id);

      expect(flattened[i].stats).toEqual(stats[i].stats);
    }
  });
});

function randomInt(max: number): number {
  return Math.floor(Math.random() * max);
}

function randomFloat(scale: number): number {
  return Math.random() * scale;
}

function randomStat(scale = 1): NumericStat {
  return {
    mean: randomFloat(scale),
    squared_diffs: randomFloat(scale * 0.3),
  };
}

function randomExecStats(count = 10): Required<ExecStats> {
  return {
    count: Long.fromNumber(randomInt(count)),
    network_bytes: randomStat(),
    max_mem_usage: randomStat(),
    contention_time: randomStat(),
    network_messages: randomStat(),
    max_disk_usage: randomStat(),
  };
}

function randomStats(
  sensitiveInfo?: ISensitiveInfo,
): Required<StatementStatistics> {
  const count = randomInt(1000);
  // tslint:disable:variable-name
  const first_attempt_count = randomInt(count);
  const max_retries = randomInt(count - first_attempt_count);
  // tslint:enable:variable-name

  return {
    count: Long.fromNumber(count),
    first_attempt_count: Long.fromNumber(first_attempt_count),
    max_retries: Long.fromNumber(max_retries),
    num_rows: randomStat(100),
    parse_lat: randomStat(),
    plan_lat: randomStat(),
    run_lat: randomStat(),
    service_lat: randomStat(),
    overhead_lat: randomStat(),
    bytes_read: randomStat(),
    rows_read: randomStat(),
    rows_written: randomStat(),
    sensitive_info: sensitiveInfo || makeSensitiveInfo(null, null),
    legacy_last_err: "",
    legacy_last_err_redacted: "",
    exec_stats: randomExecStats(count),
    sql_type: "DDL",
    last_exec_timestamp: {
      seconds: Long.fromInt(1599670292),
      nanos: 111613000,
    },
    nodes: [Long.fromInt(1), Long.fromInt(3), Long.fromInt(4)],
    plan_gists: ["Ais="],
  };
}

function randomString(length = 10): string {
  const possible =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let text = "";
  for (let i = 0; i < length; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length));
  }
  return text;
}

function randomPlanDescription(): IExplainTreePlanNode {
  return {
    name: randomString(),
    attrs: [
      {
        key: randomString(),
        value: randomString(),
      },
    ],
  };
}

describe("combineStatementStats", () => {
  it("combines statement statistics", () => {
    const a = randomStats();
    const b = randomStats();
    const c = randomStats();

    const ab = combineStatementStats([a, b]);
    const ac = combineStatementStats([a, c]);
    const bc = combineStatementStats([b, c]);

    const ab_c = combineStatementStats([ab, c]);
    const ac_b = combineStatementStats([ac, b]);
    const bc_a = combineStatementStats([bc, a]);

    expect(ab_c.count.toString()).toEqual(ac_b.count.toString());
    expect(ab_c.count.toString()).toEqual(bc_a.count.toString());

    expect(ab_c.first_attempt_count.toString()).toEqual(
      ac_b.first_attempt_count.toString(),
    );
    expect(ab_c.first_attempt_count.toString()).toEqual(
      bc_a.first_attempt_count.toString(),
    );

    expect(ab_c.max_retries.toString()).toEqual(ac_b.max_retries.toString());
    expect(ab_c.max_retries.toString()).toEqual(bc_a.max_retries.toString());

    expect(
      Math.abs(ab_c.num_rows.mean - ac_b.num_rows.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.num_rows.mean - bc_a.num_rows.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.num_rows.squared_diffs - ac_b.num_rows.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.num_rows.squared_diffs - bc_a.num_rows.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    expect(
      Math.abs(ab_c.parse_lat.mean - ac_b.parse_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.parse_lat.mean - bc_a.parse_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.parse_lat.squared_diffs - ac_b.parse_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.parse_lat.squared_diffs - bc_a.parse_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    expect(
      Math.abs(ab_c.plan_lat.mean - ac_b.plan_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.plan_lat.mean - bc_a.plan_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.plan_lat.squared_diffs - ac_b.plan_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.plan_lat.squared_diffs - bc_a.plan_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    expect(Math.abs(ab_c.run_lat.mean - ac_b.run_lat.mean)).toBeLessThanOrEqual(
      0.0000001,
    );
    expect(Math.abs(ab_c.run_lat.mean - bc_a.run_lat.mean)).toBeLessThanOrEqual(
      0.0000001,
    );
    expect(
      Math.abs(ab_c.run_lat.squared_diffs - ac_b.run_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.run_lat.squared_diffs - bc_a.run_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    expect(
      Math.abs(ab_c.service_lat.mean - ac_b.service_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.service_lat.mean - bc_a.service_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.service_lat.squared_diffs - ac_b.service_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.service_lat.squared_diffs - bc_a.service_lat.squared_diffs),
    ).toBeLessThanOrEqual(0.0000001);

    expect(
      Math.abs(ab_c.overhead_lat.mean - ac_b.overhead_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(ab_c.overhead_lat.mean - bc_a.overhead_lat.mean),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(
        ab_c.overhead_lat.squared_diffs - ac_b.overhead_lat.squared_diffs,
      ),
    ).toBeLessThanOrEqual(0.0000001);
    expect(
      Math.abs(
        ab_c.overhead_lat.squared_diffs - bc_a.overhead_lat.squared_diffs,
      ),
    ).toBeLessThanOrEqual(0.0000001);
  });

  describe("when sensitiveInfo has data", () => {
    it("uses first non-empty property from each statementStat", () => {
      const error1 = randomString();
      const error2 = randomString();
      const plan1 = randomPlanDescription();
      const plan2 = randomPlanDescription();

      const empty = makeSensitiveInfo(null, null);
      const a = makeSensitiveInfo(error1, null);
      const b = makeSensitiveInfo(null, plan1);
      const c = makeSensitiveInfo(error2, plan2);

      function assertSensitiveInfoInCombineStatementStats(
        input: ISensitiveInfo[],
        expected: ISensitiveInfo,
      ) {
        const stats = input.map(sensitiveInfo => randomStats(sensitiveInfo));
        const result = combineStatementStats(stats);
        expect(result.sensitive_info).toEqual(expected);
      }

      assertSensitiveInfoInCombineStatementStats([empty], empty);
      assertSensitiveInfoInCombineStatementStats([a], a);
      assertSensitiveInfoInCombineStatementStats([b], b);
      assertSensitiveInfoInCombineStatementStats([c], c);

      assertSensitiveInfoInCombineStatementStats([empty, a], a);
      assertSensitiveInfoInCombineStatementStats([empty, b], b);
      assertSensitiveInfoInCombineStatementStats([empty, c], c);
      assertSensitiveInfoInCombineStatementStats([a, empty], a);
      assertSensitiveInfoInCombineStatementStats([b, empty], b);
      assertSensitiveInfoInCombineStatementStats([c, empty], c);

      assertSensitiveInfoInCombineStatementStats([a, b, c], {
        last_err: a.last_err,
        most_recent_plan_description: b.most_recent_plan_description,
      });
      assertSensitiveInfoInCombineStatementStats([a, c, b], {
        last_err: a.last_err,
        most_recent_plan_description: c.most_recent_plan_description,
      });
      assertSensitiveInfoInCombineStatementStats([b, c, a], {
        last_err: c.last_err,
        most_recent_plan_description: b.most_recent_plan_description,
      });
      assertSensitiveInfoInCombineStatementStats([c, a, b], c);
    });
  });
});
