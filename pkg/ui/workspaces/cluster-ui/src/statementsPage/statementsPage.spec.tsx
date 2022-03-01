// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import React from "react";
import { ReactWrapper, mount } from "enzyme";
import { MemoryRouter } from "react-router-dom";

import {
  filterBySearchQuery,
  StatementsPage,
  StatementsPageProps,
  StatementsPageState,
} from "src/statementsPage";
import statementsPagePropsFixture from "./statementsPage.fixture";
import { AggregateStatistics } from "../statementsTable";
import { FlatPlanNode } from "../statementDetails";

describe("StatementsPage", () => {
  describe("Statements table", () => {
    it("sorts data by Execution Count DESC as default option", () => {
      const rootWrapper = mount(
        <MemoryRouter>
          <StatementsPage {...statementsPagePropsFixture} />
        </MemoryRouter>,
      );

      const statementsPageWrapper: ReactWrapper<
        StatementsPageProps,
        StatementsPageState,
        React.Component<any, any>
      > = rootWrapper.find(StatementsPage).first();
      const statementsPageInstance = statementsPageWrapper.instance();

      expect(statementsPageInstance.props.sortSetting.columnTitle).toEqual(
        "executionCount",
      );
      expect(statementsPageInstance.props.sortSetting.ascending).toEqual(false);
    });
  });

  describe("filterBySearchQuery", () => {
    const testPlanNode: FlatPlanNode = {
      name: "render",
      attrs: [],
      children: [
        {
          name: "group (scalar)",
          attrs: [],
          children: [
            {
              name: "filter",
              attrs: [
                {
                  key: "filter",
                  values: ["variable = _"],
                  warn: false,
                },
              ],
              children: [
                {
                  name: "virtual table",
                  attrs: [
                    {
                      key: "table",
                      values: ["cluster_settings@primary"],
                      warn: false,
                    },
                  ],
                  children: [],
                },
              ],
            },
          ],
        },
      ],
    };

    const statement: AggregateStatistics = {
      aggregatedFingerprintID: "",
      aggregatedTs: 0,
      aggregationInterval: 0,
      database: "",
      fullScan: false,
      implicitTxn: false,
      summary: "",
      label:
        "SELECT count(*) > _ FROM [SHOW ALL CLUSTER SETTINGS] AS _ (v) WHERE v = '_'",
      stats: {
        sensitive_info: {
          most_recent_plan_description: testPlanNode,
        },
      },
    };

    expect(filterBySearchQuery(statement, "select")).toEqual(true);
    expect(filterBySearchQuery(statement, "virtual table")).toEqual(true);
    expect(filterBySearchQuery(statement, "group (scalar)")).toEqual(true);
    expect(filterBySearchQuery(statement, "node_build_info")).toEqual(false);
    expect(filterBySearchQuery(statement, "crdb_internal")).toEqual(false);
  });
});
