import React from "react";

export function moduleWithStubbedComponent(path: string, exportedName?: string) {
  const orig = jest.requireActual(path);
  let target: unknown  = orig;
  if (typeof orig === "object" && exportedName && orig.hasOwnProperty(exportedName)) {
    console.log(`Found component ${exportedName} in ${path}: ${(orig as any)[exportedName]}`);
    target = (orig as any)[exportedName];
  }

  const stubRender = jest.fn(() => <div data-componentname={target.constructor.name}/> as any);
  let found = false;
  if (typeof target === "function") {
    console.log("target is a function: ");
    found = true;
    target = stubRender;
  }

  if (found) {
    if (orig === target) {
      return target;
    }

    if (orig.hasOwnProperty(exportedName)) {
      orig[exportedName] = target;
      return orig;
    }
  }

  throw new Error(`Unable to mock '${path}' property '${exportedName}', which has type '${typeof target}'`);
}

export function mockModlueWithStubbedComponent(path: string, exportedName?: string) {
  jest.doMock(path, () => {
    const out = moduleWithStubbedComponent(path, exportedName)
    console.log("[mockModlueWithStubbedComponent::factory] producing", out);
    return out;
  });
}
