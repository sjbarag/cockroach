// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import {
  filterTransactions,
  getStatementsByFingerprintIdAndTime,
  statementFingerprintIdsToText,
} from "./utils";
import { TimestampToString } from "../util";
import { Filters } from "../queryFilter";
import { data, nodeRegions, timestamp } from "./transactions.fixture";
import Long from "long";
import * as protos from "@cockroachlabs/crdb-protobuf-client";

type Transaction = protos.cockroach.server.serverpb.StatementsResponse.IExtendedCollectedTransactionStatistics;

describe("getStatementsByFingerprintIdAndTime", () => {
  it("filters statements by fingerprint id and time", () => {
    const selectedStatements = getStatementsByFingerprintIdAndTime(
      [Long.fromInt(4104049045071304794), Long.fromInt(3334049045071304794)],
      TimestampToString(timestamp),
      [
        {
          id: Long.fromInt(4104049045071304794),
          key: { aggregated_ts: timestamp },
        },
        { id: Long.fromInt(5554049045071304794) },
      ],
    );
    expect(selectedStatements.length).toBe(1);
    expect(selectedStatements[0].id.eq(Long.fromInt(4104049045071304794))).toBe(
      true,
    );
  });
});

const txData = data.transactions as Transaction[];

describe("Filter transactions", () => {
  it("show non internal if no filters applied", () => {
    const filter: Filters = {
      app: "",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(4);
  });

  it("filters by app", () => {
    const filter: Filters = {
      app: "$ TEST",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(3);
  });

  it("filters by app exactly", () => {
    const filter: Filters = {
      app: "$ TEST EXACT",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(1);
  });

  it("filters by 2 apps", () => {
    const filter: Filters = {
      app: "$ TEST EXACT,$ TEST",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(4);
  });

  it("filters by internal prefix", () => {
    const filter: Filters = {
      app: data.internal_app_name_prefix,
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(7);
  });

  it("filters by time", () => {
    const filter: Filters = {
      app: "$ internal,$ TEST",
      timeNumber: "40",
      timeUnit: "miliseconds",
      nodes: "",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(8);
  });

  it("filters by one node", () => {
    const filter: Filters = {
      app: "$ internal,$ TEST",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "n1",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(6);
  });

  it("filters by multiple nodes", () => {
    const filter: Filters = {
      app: "$ internal,$ TEST,$ TEST EXACT",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "n2,n4",
      regions: "",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(8);
  });

  it("filters by one region", () => {
    const filter: Filters = {
      app: "$ internal,$ TEST",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "gcp-europe-west1",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(4);
  });

  it("filters by multiple regions", () => {
    const filter: Filters = {
      app: "$ internal,$ TEST,$ TEST EXACT",
      timeNumber: "0",
      timeUnit: "seconds",
      nodes: "",
      regions: "gcp-us-west1,gcp-europe-west1",
    };
    expect(
      filterTransactions(
        txData,
        filter,
        "$ internal",
        data.statements,
        nodeRegions,
        false,
      ).transactions.length,
    ).toEqual(9);
  });
});

describe("statementFingerprintIdsToText", () => {
  it("translate statement fingerprint IDs into queries", () => {
    const statements = [
      {
        id: Long.fromInt(4104049045071304794),
        key: {
          key_data: {
            query: "SELECT _",
          },
        },
      },
      {
        id: Long.fromInt(5104049045071304794),
        key: {
          key_data: {
            query: "SELECT _, _",
          },
        },
      },
    ];
    const statementFingerprintIds = [
      Long.fromInt(4104049045071304794),
      Long.fromInt(5104049045071304794),
      Long.fromInt(4104049045071304794),
      Long.fromInt(4104049045071304794),
    ];

    expect(statementFingerprintIdsToText(statementFingerprintIds, statements))
      .toEqual(`SELECT _
SELECT _, _
SELECT _
SELECT _`);
  });
});
