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
import { mount } from "enzyme";
import { MemoryRouter as Router } from "react-router-dom";
import { StatementDetails, StatementDetailsProps } from "./statementDetails";
import { DiagnosticsView } from "./diagnostics/diagnosticsView";
import { getStatementDetailsPropsFixture } from "./statementDetails.fixture";
import { Loading } from "../loading";

describe("StatementDetails page", () => {
  let statementDetailsProps: StatementDetailsProps;

  beforeEach(() => {
    statementDetailsProps = getStatementDetailsPropsFixture();
  });

  it("shows loading indicator when data is not ready yet", () => {
    statementDetailsProps.statement = null;
    statementDetailsProps.statementsError = null;

    const wrapper = mount(
      <Router>
        <StatementDetails {...statementDetailsProps} />
      </Router>,
    );
    expect(wrapper.find(Loading).prop("loading")).toBe(true);
    expect(
      wrapper
        .find(StatementDetails)
        .find("div.ant-tabs-tab")
        .exists(),
    ).toBe(false);
  });

  it("shows error alert when `lastError` is not null", () => {
    statementDetailsProps.statementsError = new Error("Something went wrong");

    const wrapper = mount(
      <Router>
        <StatementDetails {...statementDetailsProps} />
      </Router>,
    );
    expect(wrapper.find(Loading).prop("error")).not.toBeNull();
    expect(
      wrapper
        .find(StatementDetails)
        .find("div.ant-tabs-tab")
        .exists(),
    ).toBe(false);
  });

  it("calls onTabChanged prop when selected tab is changed", () => {
    const onTabChangeSpy = jest.fn();
    const wrapper = mount(
      <Router>
        <StatementDetails
          {...statementDetailsProps}
          onTabChanged={onTabChangeSpy}
        />
      </Router>,
    );

    wrapper
      .find(StatementDetails)
      .find("div.ant-tabs-tab")
      .last()
      .simulate("click");

    expect(onTabChangeSpy).toBeCalledWith("execution-stats");
  });

  describe("Diagnostics tab", () => {
    beforeEach(() => {
      statementDetailsProps.history.location.search = new URLSearchParams([
        ["tab", "diagnostics"],
      ]).toString();
    });

    // FIXME(barag) - sinon-based test didn't assert anything on the result of calledOnceWith
    it.skip("calls createStatementDiagnosticsReport callback on Activate button click", () => {
      const onDiagnosticsActivateClickSpy = jest.fn();
      const wrapper = mount(
        <Router>
          <StatementDetails
            {...statementDetailsProps}
            createStatementDiagnosticsReport={onDiagnosticsActivateClickSpy}
          />
        </Router>,
      );

      wrapper
        .find(DiagnosticsView)
        .findWhere(n => n.prop("children") === "Activate Diagnostics")
        .first()
        .simulate("click");

      expect(onDiagnosticsActivateClickSpy).toBeCalledTimes(1);
      expect(onDiagnosticsActivateClickSpy).toBeCalledWith(
        statementDetailsProps.statement.statement,
      );
    });
  });
});
