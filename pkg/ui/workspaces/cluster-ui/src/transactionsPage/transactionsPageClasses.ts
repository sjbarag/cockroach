// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import classNames from "classnames/bind";
import statementsPageStyles from "../statementsPage/statementsPage.module.scss";
import sortedTableStyles from "../sortedtable/sortedtable.module.scss";
import { commonStyles } from "../common";

const pageCx = classNames.bind(statementsPageStyles);
const sortedTableCx = classNames.bind(sortedTableStyles);

export const baseHeadingClasses = {
  wrapper: pageCx("section--heading"),
  tableName: commonStyles("base-heading", "no-margin-bottom"),
};

export const statisticsClasses = {
  statistic: pageCx("cl-table-statistic"),
  countTitle: pageCx("cl-count-title"),
  tableContainerClass: sortedTableCx("cl-table-container"),
};
