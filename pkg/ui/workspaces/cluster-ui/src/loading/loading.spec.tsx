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
import { Spinner, InlineAlert } from "@cockroachlabs/ui-components";
import { Loading } from "./loading";

const SomeComponent = () => <div>Hello, world!</div>;
const SomeCustomErrorComponent = () => <div>Custom Error</div>;

describe("<Loading>", () => {
  describe("when error is null", () => {
    describe("when loading=false", () => {
      it("renders content.", () => {
        const wrapper = mount(
          <Loading
            loading={false}
            page={"Test"}
            error={null}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(true);
      });
    });

    describe("when loading=true", () => {
      it("renders loading spinner.", () => {
        const wrapper = mount(
          <Loading
            loading={true}
            page={"Test"}
            error={null}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(true);
      });
    });
  });

  describe("when error is a single error", () => {
    describe("when loading=false", () => {
      it("renders error, regardless of loading value.", () => {
        const wrapper = mount(
          <Loading
            loading={false}
            page={"Test"}
            error={Error("some error message")}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(InlineAlert).exists()).toBe(true);
      });
    });

    describe("when loading=true", () => {
      it("renders error, regardless of loading value.", () => {
        const wrapper = mount(
          <Loading
            loading={true}
            page={"Test"}
            error={Error("some error message")}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(SomeCustomErrorComponent).exists()).toBe(false);
        expect(wrapper.find(InlineAlert).exists()).toBe(true);
      });

      it("render custom error when provided", () => {
        const wrapper = mount(
          <Loading
            loading={true}
            page={"Test"}
            error={Error("some error message")}
            render={() => <SomeComponent />}
            renderError={() => <SomeCustomErrorComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(SomeCustomErrorComponent).exists()).toBe(true);
      });
    });
  });

  describe("when error is a list of errors", () => {
    describe("when no errors are null", () => {
      it("renders all errors in list", () => {
        const errors = [Error("error1"), Error("error2"), Error("error3")];
        const wrapper = mount(
          <Loading
            loading={false}
            page={"Test"}
            error={errors}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(InlineAlert).exists()).toBe(true);
        errors.forEach(e =>
          expect(
            wrapper
              .find(InlineAlert)
              .text()
              .includes(e.message),
          ).toBe(true),
        );
      });
    });

    describe("when some errors are null", () => {
      it("ignores null list values, rending only valid errors.", () => {
        const errors = [
          null,
          Error("error1"),
          Error("error2"),
          null,
          Error("error3"),
          null,
        ];
        const wrapper = mount(
          <Loading
            loading={false}
            page={"Test"}
            error={errors}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(false);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(InlineAlert).exists()).toBe(true);
        errors
          .filter(e => !!e)
          .forEach(e =>
            expect(
              wrapper
                .find(InlineAlert)
                .text()
                .includes(e.message),
            ).toBe(true),
          );
      });
    });

    describe("when all errors are null", () => {
      it("renders content, since there are no errors.", () => {
        const wrapper = mount(
          <Loading
            loading={false}
            page={"Test"}
            error={[null, null, null]}
            render={() => <SomeComponent />}
          />,
        );
        expect(wrapper.find(SomeComponent).exists()).toBe(true);
        expect(wrapper.find(Spinner).exists()).toBe(false);
        expect(wrapper.find(InlineAlert).exists()).toBe(false);
      });
    });
  });
});
