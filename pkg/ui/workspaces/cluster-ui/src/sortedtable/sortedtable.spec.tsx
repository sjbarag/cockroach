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
import _ from "lodash";
import { mount, ReactWrapper } from "enzyme";
import classNames from "classnames/bind";
import {
  SortedTable,
  ColumnDescriptor,
  ISortedTablePagination,
  SortSetting,
} from "src/sortedtable";
import styles from "src/sortabletable/sortabletable.module.scss";

const cx = classNames.bind(styles);

class TestRow {
  constructor(public name: string, public value: number) {}
}

const columns: ColumnDescriptor<TestRow>[] = [
  {
    name: "first",
    title: "first",
    cell: tr => tr.name,
    sort: tr => tr.name,
  },
  {
    name: "second",
    title: "second",
    cell: tr => tr.value.toString(),
    sort: tr => tr.value,
    rollup: trs => _.sumBy(trs, tr => tr.value),
  },
];

class TestSortedTable extends SortedTable<TestRow> {}

function makeTable(
  data: TestRow[],
  sortSetting?: SortSetting,
  onChangeSortSetting?: (ss: SortSetting) => void,
  pagination?: ISortedTablePagination,
) {
  return mount(
    <TestSortedTable
      data={data}
      sortSetting={sortSetting}
      onChangeSortSetting={onChangeSortSetting}
      pagination={pagination}
      columns={columns}
    />,
  );
}

function makeExpandableTable(data: TestRow[], sortSetting: SortSetting) {
  return mount(
    <TestSortedTable
      data={data}
      columns={columns}
      sortSetting={sortSetting}
      expandableConfig={{
        expandedContent: testRow => (
          <div>
            {testRow.name}={testRow.value}
          </div>
        ),
        expansionKey: testRow => testRow.name,
      }}
    />,
  );
}

function rowsOf(wrapper: ReactWrapper): Array<Array<string>> {
  return wrapper.find("tr").map(tr => tr.find("td").map(td => td.text()));
}

describe("<SortedTable>", function() {
  it("renders the expected table structure.", function() {
    const wrapper = makeTable([new TestRow("test", 1)]);
    expect(wrapper.find("table").length).toBe(1);
    expect(wrapper.find("thead").find("tr").length).toBe(1);
    expect(wrapper.find(`tr.${cx("head-wrapper__row--header")}`).length).toBe(
      1,
    );
    expect(wrapper.find("tbody").length).toBe(1);
  });

  it("correctly uses onChangeSortSetting", function() {
    const spy = jest.fn();
    const wrapper = makeTable([new TestRow("test", 1)], undefined, spy);
    wrapper
      .find(`th.${cx("head-wrapper__cell")}`)
      .first()
      .simulate("click");
    expect(spy).toBeCalledTimes(1);
    expect(spy).toHaveBeenCalledWith({
      ascending: false,
      columnTitle: "first",
    } as SortSetting);
  });

  it("correctly sorts data based on sortSetting", function() {
    const data = [
      new TestRow("c", 3),
      new TestRow("d", 4),
      new TestRow("a", 1),
      new TestRow("b", 2),
    ];
    let wrapper = makeTable(data, undefined);
    const assertMatches = (expected: TestRow[]) => {
      const rows = wrapper.find("tbody");
      _.each(expected, (rowData, dataIndex) => {
        const row = rows.childAt(dataIndex);
        expect(
          row
            .childAt(0)
            .childAt(0)
            .text(),
        ).toEqual(rowData.name);
        expect(
          row
            .childAt(0)
            .childAt(1)
            .text(),
        ).toEqual(rowData.value.toString());
      });
    };
    assertMatches(data);
    wrapper = makeTable(data, {
      ascending: true,
      columnTitle: "first",
    });
    assertMatches(_.sortBy(data, r => r.name));
    wrapper.setProps({
      uiSortSetting: {
        ascending: true,
        columnTitle: "second",
      } as SortSetting,
    });
    assertMatches(_.sortBy(data, r => r.value));
  });

  describe("with expandableConfig", function() {
    it("renders the expected table structure", function() {
      const wrapper = makeExpandableTable([new TestRow("test", 1)], undefined);
      expect(wrapper.find("table").length).toBe(1);
      expect(wrapper.find("thead").find("tr").length).toBe(1);
      expect(wrapper.find(`tr.${cx("head-wrapper__row--header")}`).length).toBe(
        1,
      );
      expect(wrapper.find("tbody").length).toBe(1);
      expect(wrapper.find("tbody tr").length).toBe(1);
      expect(wrapper.find("tbody td").length).toBe(3);
      expect(
        wrapper.find(`td.${cx("row-wrapper__cell__expansion-control")}`).length,
      ).toBe(1);
    });

    it("expands and collapses the clicked row", function() {
      const wrapper = makeExpandableTable([new TestRow("test", 1)], undefined);
      expect(
        wrapper.find(`.${cx("row-wrapper__row--expanded-area")}`).length,
      ).toBe(0);
      wrapper
        .find(`.${cx("row-wrapper__cell__expansion-control")}`)
        .simulate("click");
      const expandedArea = wrapper.find(".row-wrapper__row--expanded-area");
      expect(expandedArea.length).toBe(1);
      expect(expandedArea.children().length).toBe(2);
      expect(expandedArea.contains(<td />)).toBe(true);
      expect(
        expandedArea.contains(
          <td className={cx("row-wrapper__cell")} colSpan={2}>
            <div>test=1</div>
          </td>,
        ),
      ).toBe(true);
      wrapper
        .find(`.${cx("row-wrapper__cell__expansion-control")}`)
        .simulate("click");
      expect(
        wrapper.find(`.${cx("row-wrapper__row--expanded-area")}`).length,
      ).toBe(0);
    });
  });

  it("should correctly render rows with pagination and sort settings", function() {
    const data = [
      new TestRow("c", 3),
      new TestRow("d", 4),
      new TestRow("a", 1),
      new TestRow("b", 2),
    ];
    let wrapper = makeTable(data, undefined, undefined, {
      current: 1,
      pageSize: 2,
    });
    let rows = wrapper.find("tbody");
    expect(wrapper.find("tbody tr").length).toBe(2);
    expect(
      rows
        .childAt(1)
        .childAt(0)
        .childAt(0)
        .text(),
    ).toEqual("d");

    wrapper = makeTable(data, undefined, undefined, {
      current: 2,
      pageSize: 2,
    });
    rows = wrapper.find("tbody");
    expect(wrapper.find("tbody tr").length).toBe(2);
    expect(
      rows
        .childAt(0)
        .childAt(0)
        .childAt(0)
        .text(),
    ).toEqual("a");

    wrapper = makeTable(
      data,
      { ascending: true, columnTitle: "first" },
      undefined,
      {
        current: 1,
        pageSize: 2,
      },
    );
    rows = wrapper.find("tbody");
    expect(
      rows
        .childAt(1)
        .childAt(0)
        .childAt(0)
        .text(),
    ).toEqual("b");

    wrapper = makeTable(
      data,
      { ascending: true, columnTitle: "first" },
      undefined,
      {
        current: 2,
        pageSize: 2,
      },
    );
    rows = wrapper.find("tbody");
    expect(
      rows
        .childAt(0)
        .childAt(0)
        .childAt(0)
        .text(),
    ).toEqual("c");
  });

  it("should update when pagination changes", function() {
    const table = makeTable(
      [
        new TestRow("c", 3),
        new TestRow("d", 4),
        new TestRow("a", 1),
        new TestRow("b", 2),
      ],
      undefined,
      undefined,
      {
        current: 1,
        pageSize: 2,
      },
    );

    expect(rowsOf(table.find("tbody"))).toEqual([
      ["c", "3"],
      ["d", "4"],
    ]);

    table.setProps({ pagination: { current: 2, pageSize: 2 } });

    expect(rowsOf(table.find("tbody"))).toEqual([
      ["a", "1"],
      ["b", "2"],
    ]);
  });
});
