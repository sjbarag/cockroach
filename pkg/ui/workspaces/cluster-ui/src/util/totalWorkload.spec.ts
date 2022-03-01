// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import { calculateTotalWorkload } from "./totalWorkload";
import { aggStatFix } from "./totalWorkload.fixture";

describe("Calculating total workload", () => {
  it("calculating total workload with one statement", () => {
    const result = calculateTotalWorkload([aggStatFix]);
    // Using approximately because float handling by javascript is imprecise
    expect(Math.abs(result - 48.421019)).toBeLessThanOrEqual(0.0000001);
  });

  it("calculating total workload with no statements", () => {
    const result = calculateTotalWorkload([]);
    expect(result).toEqual(0);
  });

  it("calculating total workload with multiple statements", () => {
    const result = calculateTotalWorkload([aggStatFix, aggStatFix, aggStatFix]);
    // Using approximately because float handling by javascript is imprecise
    expect(Math.abs(result - 145.263057)).toBeLessThanOrEqual(0.0000001);
  });
});
