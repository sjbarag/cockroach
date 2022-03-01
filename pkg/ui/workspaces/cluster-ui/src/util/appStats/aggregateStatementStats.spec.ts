// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import { aggregateStatementStats } from "./appStats";
import { statementsWithSameIdButDifferentNodeId } from "./appStats.fixture";

describe("aggregateStatementStats", () => {
  it("groups duplicate statements by node id", () => {
    const aggregated = aggregateStatementStats(
      statementsWithSameIdButDifferentNodeId,
    );
    expect(aggregated.length).toEqual(1);
  });
});
