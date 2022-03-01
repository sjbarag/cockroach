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
import {
  getTimeRangeTitle,
  generateDisabledArrows,
  timeFormat,
  dateFormat,
  TimeScaleDropdownProps,
  TimeScaleDropdown,
} from "./timeScaleDropdown";
import { defaultTimeScaleOptions, findClosestTimeScale } from "./utils";
import * as timescale from "./timeScaleTypes";
import moment from "moment";
import { MemoryRouter } from "react-router";
import TimeFrameControls from "./timeFrameControls";
import RangeSelect from "./rangeSelect";
import FakeTimers from "@sinonjs/fake-timers";
import { TimeWindow, ArrowDirection, TimeScale } from "./timeScaleTypes";

const initialEntries = [
  "#/metrics/overview/cluster", // Past 10 minutes
  `#/metrics/overview/cluster/cluster?start=${moment()
    .subtract(1, "hour")
    .format("X")}&end=${moment().format("X")}`, // Past 1 hour
  `#/metrics/overview/cluster/cluster?start=${moment()
    .subtract(6, "hours")
    .format("X")}&end=${moment().format("X")}`, // Past 6 hours
  "#/metrics/overview/cluster/cluster?start=1584528492&end=1584529092", // 10 minutes
  "#/metrics/overview/cluster?start=1583319565&end=1584529165", // 2 weeks
  "#/metrics/overview/node/1", // Node 1 - Past 10 minutes
  `#/metrics/overview/node/2?start=${moment()
    .subtract(10, "minutes")
    .format("X")}&end=${moment().format("X")}`, // Node 2 - Past 10 minutes
  "#/metrics/overview/node/3?start=1584528726&end=1584529326", // Node 3 - 10 minutes
];

describe("<TimeScaleDropdown>", function() {
  let state: TimeScaleDropdownProps;
  let clock: FakeTimers.InstalledClock;
  let currentWindow: TimeWindow;

  const setCurrentWindowFromTimeScale = (timeScale: TimeScale): void => {
    const end = timeScale.fixedWindowEnd || moment.utc();
    currentWindow = {
      start: moment(end).subtract(timeScale.windowSize),
      end,
    };
  };

  const makeTimeScaleDropdown = (props: TimeScaleDropdownProps) => {
    setCurrentWindowFromTimeScale(props.currentScale);
    return mount(
      <MemoryRouter initialEntries={initialEntries}>
        <TimeScaleDropdown {...props} />
      </MemoryRouter>,
    );
  };

  beforeEach(() => {
    clock = FakeTimers.install({
      now: new Date(2020, 5, 1, 9, 28, 30),
    });
    const timeScaleState = new timescale.TimeScaleState();
    setCurrentWindowFromTimeScale(timeScaleState.scale);
    state = {
      currentScale: timeScaleState.scale,
      setTimeScale: () => {},
    };
  });

  afterEach(() => {
    clock.uninstall();
  });

  it("valid path should not redirect to 404", () => {
    const wrapper = makeTimeScaleDropdown(state);
    expect(wrapper.find(RangeSelect).length).toEqual(1);
    expect(wrapper.find(TimeFrameControls).length).toEqual(1);
  });

  it("Past 10 minutes must be render", () => {
    const wrapper = makeTimeScaleDropdown(state);
    wrapper.setProps({ currentScale: state.currentScale });
    const expected: TimeScale = {
      key: "Past 10 Minutes",
      ...defaultTimeScaleOptions["Past 10 Minutes"],
      fixedWindowEnd: false,
    };
    expect(wrapper.props().currentScale).toEqual(expected);
  });

  it("getTimeRangeTitle must return title Past 10 Minutes", () => {
    const wrapper = makeTimeScaleDropdown(state);
    expect(
      wrapper
        .find(".trigger .Select-value-label")
        .first()
        .text(),
    ).toEqual("Past 10 Minutes");

    const title = getTimeRangeTitle(currentWindow, state.currentScale);
    expect(title).toEqual({ title: "Past 10 Minutes", timeLabel: "10m" });
  });

  describe("getTimeRangeTitle", () => {
    it("returns custom Title with Time part only for current day", () => {
      const currentScale = { ...state.currentScale, key: "Custom" };
      const title = getTimeRangeTitle(currentWindow, currentScale);
      const timeStart = moment.utc(currentWindow.start).format(timeFormat);
      const timeEnd = moment.utc(currentWindow.end).format(timeFormat);
      const wrapper = makeTimeScaleDropdown({ ...state, currentScale });
      expect(
        wrapper
          .find(".trigger .Select-value-label")
          .first()
          .text(),
      ).toEqual(` ${timeStart} -  ${timeEnd} (UTC)`);
      expect(title).toEqual({
        dateStart: "",
        dateEnd: "",
        timeStart,
        timeEnd,
        title: "Custom",
        timeLabel: "10m",
      });
    });

    it("returns custom Title with Date and Time part for the range with past days", () => {
      const window: TimeWindow = {
        start: moment(currentWindow.start).subtract(2, "day"),
        end: moment(currentWindow.end).subtract(1, "day"),
      };
      const currentScale = {
        ...state.currentScale,
        fixedWindowEnd: window.end,
        windowSize: moment.duration(
          window.end.diff(window.start, "seconds"),
          "seconds",
        ),
        key: "Custom",
      };
      const title = getTimeRangeTitle(window, currentScale);
      const timeStart = moment.utc(window.start).format(timeFormat);
      const timeEnd = moment.utc(window.end).format(timeFormat);
      const dateStart = moment.utc(window.start).format(dateFormat);
      const dateEnd = moment.utc(window.end).format(dateFormat);
      const wrapper = makeTimeScaleDropdown({
        ...state,
        currentScale,
      });
      expect(
        wrapper
          .find(".trigger .Select-value-label")
          .first()
          .text(),
      ).toEqual(`${dateStart} ${timeStart} - ${dateEnd} ${timeEnd} (UTC)`);
      expect(title).toEqual({
        dateStart,
        dateEnd,
        timeStart,
        timeEnd,
        title: "Custom",
        timeLabel: "1d",
      });
    });
  });

  it("generateDisabledArrows must return array with disabled buttons", () => {
    const arrows = generateDisabledArrows(currentWindow);
    const wrapper = makeTimeScaleDropdown(state);
    expect(wrapper.find(".controls-content ._action.disabled").length).toEqual(
      2,
    );
    expect(arrows).toEqual([ArrowDirection.CENTER, ArrowDirection.RIGHT]);
  });

  it("generateDisabledArrows must render 3 active buttons and return empty array", () => {
    const window: TimeWindow = {
      start: moment(currentWindow.start).subtract(1, "day"),
      end: moment(currentWindow.end).subtract(1, "day"),
    };
    const currentTimeScale = {
      ...state.currentScale,
      fixedWindowEnd: window.end,
    };
    const arrows = generateDisabledArrows(window);
    const wrapper = makeTimeScaleDropdown({
      ...state,
      currentScale: currentTimeScale,
    });
    expect(wrapper.find(".controls-content ._action.disabled").length).toEqual(
      0,
    );
    expect(arrows).toEqual([]);
  });
});

describe("timescale utils", (): void => {
  describe("findClosestTimeScale", () => {
    it("should find the correct time scale", () => {
      // `seconds` != window size of any of the default options, `startSeconds` not specified.
      expect(findClosestTimeScale(defaultTimeScaleOptions, 15)).toEqual({
        ...defaultTimeScaleOptions["Past 10 Minutes"],
        key: "Custom",
      });
      // `seconds` != window size of any of the default options, `startSeconds` not specified.
      expect(
        findClosestTimeScale(
          defaultTimeScaleOptions,
          moment.duration(moment().daysInMonth() * 5, "days").asSeconds(),
        ),
      ).toEqual({ ...defaultTimeScaleOptions["Past 2 Months"], key: "Custom" });
      // `seconds` == window size of one of the default options, `startSeconds` not specified.
      expect(
        findClosestTimeScale(
          defaultTimeScaleOptions,
          moment.duration(10, "minutes").asSeconds(),
        ),
      ).toEqual({
        ...defaultTimeScaleOptions["Past 10 Minutes"],
        key: "Past 10 Minutes",
      });
      // `seconds` == window size of one of the default options, `startSeconds` not specified.
      expect(
        findClosestTimeScale(
          defaultTimeScaleOptions,
          moment.duration(14, "days").asSeconds(),
        ),
      ).toEqual({
        ...defaultTimeScaleOptions["Past 2 Weeks"],
        key: "Past 2 Weeks",
      });
      // `seconds` == window size of one of the default options, `startSeconds` is now.
      expect(
        findClosestTimeScale(
          defaultTimeScaleOptions,
          defaultTimeScaleOptions["Past 10 Minutes"].windowSize.asSeconds(),
          moment().unix(),
        ),
      ).toEqual({
        ...defaultTimeScaleOptions["Past 10 Minutes"],
        key: "Past 10 Minutes",
      });
      // `seconds` == window size of one of the default options, `startSeconds` is in the past.
      expect(
        findClosestTimeScale(
          defaultTimeScaleOptions,
          defaultTimeScaleOptions["Past 10 Minutes"].windowSize.asSeconds(),
          moment()
            .subtract(1, "day")
            .unix(),
        ),
      ).toEqual({
        ...defaultTimeScaleOptions["Past 10 Minutes"],
        key: "Custom",
      });
    });
  });
});
