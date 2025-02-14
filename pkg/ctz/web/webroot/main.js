/*
 * ATTENTION: The "eval" devtool has been used (maybe by default in mode: "development").
 * This devtool is neither made for production nor for readable output files.
 * It uses "eval()" calls to create a separate source file in the browser devtools.
 * If you are trying to read the output file, select a different devtool (https://webpack.js.org/configuration/devtool/)
 * or disable the default devtool with "devtool: false".
 * If you are looking for production-ready output files, see mode: "production" (https://webpack.js.org/configuration/mode/).
 */
/******/ (() => { // webpackBootstrap
/******/ 	"use strict";
/******/ 	var __webpack_modules__ = ({

/***/ "./src/ts/chooser.ts":
/*!***************************!*\
  !*** ./src/ts/chooser.ts ***!
  \***************************/
/***/ ((__unused_webpack_module, exports) => {

eval("\nObject.defineProperty(exports, \"__esModule\", ({ value: true }));\nexports.Chooser = void 0;\n/**\n * Presents a list of things that a user can choose from, and performs some action when one is selected.\n */\nclass Chooser {\n    callback;\n    // All values to display\n    values;\n    // Currently selected value, by key\n    selection;\n    prevSel;\n    // Current key to value mapping\n    kvMap;\n    // Map from keys to elements\n    valueElementMap;\n    // The full element\n    fullElement;\n    // The header area\n    headerArea;\n    // The item chooser area\n    chooserArea;\n    _hasInitialData = false;\n    constructor(title, callback) {\n        this.callback = callback;\n        this.selection = null;\n        this.prevSel = null;\n        this.values = [];\n        this.valueElementMap = new Map();\n        this.kvMap = new Map();\n        this.fullElement = document.createElement(\"div\");\n        this.fullElement.classList.add(\"chooser\");\n        this.chooserArea = document.createElement(\"div\");\n        this.chooserArea.classList.add(\"content-area\");\n        this.headerArea = document.createElement(\"div\");\n        this.headerArea.classList.add(\"header-area\");\n        this.headerArea.replaceChildren(...title);\n        this.fullElement.replaceChildren(this.headerArea, this.chooserArea);\n    }\n    makeItemUi(key) {\n        const out = this.makeListItemUi();\n        const outer = this;\n        out.element.classList.add(\"chooser-item\");\n        out.element.addEventListener('click', (e) => {\n            outer.setSelectionByKey(key);\n        });\n        return out;\n    }\n    ;\n    get allValues() {\n        return [...this.values];\n    }\n    setSelectionByKey(selKey, explicit = true) {\n        if (selKey === null) {\n            this.setSelection(null, explicit);\n        }\n        else {\n            const value = this.kvMap.get(selKey);\n            if (value === undefined) {\n                this.setSelection(null, explicit);\n            }\n            else {\n                this.setSelection({\n                    item: value,\n                    key: selKey,\n                }, explicit);\n            }\n        }\n    }\n    setSelection(sel, explicit = true) {\n        this.selection = sel;\n        this.refreshSelection(explicit);\n    }\n    set allValues(newValues) {\n        if (newValues.length > 0) {\n            if (!this._hasInitialData) {\n                const newSel = newValues[0];\n                this.selection = {\n                    item: newSel,\n                    key: this.extractKey(newSel),\n                };\n            }\n            this._hasInitialData = true;\n        }\n        // Replace values list\n        this.values = [...newValues];\n        // Create new element map, using the existing elements where they exist\n        const oldEleMap = this.valueElementMap;\n        const newEleMap = new Map();\n        const newKvMap = new Map();\n        // New children for the chooser area\n        const newElements = [];\n        let anyNew = false;\n        for (const value of newValues) {\n            // If the key for a new value is the same as the key for an old value, re-use the element\n            const key = this.extractKey(value);\n            newKvMap.set(key, value);\n            let element = oldEleMap.get(key);\n            // If no existing element, create a new one\n            if (element === undefined) {\n                element = this.makeItemUi(key);\n                anyNew = true;\n            }\n            element.formatFor(key, value);\n            // Push to the map and the new element list\n            newEleMap.set(key, element);\n            newElements.push(element.element);\n        }\n        // If something was selected before, and an equivalent item still exists, retain that selection.\n        // Otherwise, clear selection.\n        if (this.selection !== null) {\n            const newKey = this.selection.key;\n            const newEquivalent = newEleMap.get(newKey);\n            if (newEquivalent === undefined) {\n                this.selection = null;\n            }\n            const item = newValues.find(val => this.extractKey(val) === newKey);\n            if (item === undefined) {\n                // shouldn't happen\n                this.selection = null;\n            }\n            else {\n                this.selection = {\n                    item: item,\n                    key: newKey,\n                };\n            }\n        }\n        this.valueElementMap = newEleMap;\n        this.kvMap = newKvMap;\n        this.refreshSelection(false);\n        // Don't replace children if the children are the same.\n        // If the size is not equal, we know something changed. If the size is equal, but only because the same number\n        // of entries were added as were removed, then anyNew would be true.\n        if (anyNew || oldEleMap.size !== newEleMap.size) {\n            this.chooserArea.replaceChildren(...newElements);\n        }\n    }\n    refreshSelection(explicit) {\n        const oldSel = this.prevSel;\n        const newSel = this.selection;\n        let success = false;\n        try {\n            success = this.callback({\n                new: newSel,\n                old: oldSel,\n            }, explicit);\n        }\n        catch (e) {\n            console.error(e);\n        }\n        if (success) {\n            this.valueElementMap.forEach((v, k) => {\n                v.setSelected(k === newSel?.key);\n            });\n            this.prevSel = newSel;\n        }\n        else {\n            this.selection = oldSel;\n        }\n    }\n    get hasInitialData() {\n        return this._hasInitialData;\n    }\n    get selectedItem() {\n        return this.selection;\n    }\n}\nexports.Chooser = Chooser;\n\n\n//# sourceURL=webpack://ceph-to-zfs-webif/./src/ts/chooser.ts?");

/***/ }),

/***/ "./src/ts/formatters.ts":
/*!******************************!*\
  !*** ./src/ts/formatters.ts ***!
  \******************************/
/***/ ((__unused_webpack_module, exports) => {

eval("\nObject.defineProperty(exports, \"__esModule\", ({ value: true }));\nexports.fmtUnix = fmtUnix;\nexports.fmtDur = fmtDur;\nexports.fmtBytes = fmtBytes;\nfunction fmtUnix(unixSecs) {\n    return new Date(unixSecs * 1000).toLocaleString();\n}\nfunction fmtDur(durSecs) {\n    const secsPart = durSecs % 60;\n    const hours = Math.floor(durSecs / 3600);\n    const minutes = Math.floor((durSecs % 3600) / 60);\n    if (hours === 0) {\n        if (minutes === 0) {\n            return secsPart.toFixed(3);\n        }\n        else {\n            return `${minutes}:${secsPart.toFixed(3)}`;\n        }\n    }\n    else {\n        return `${hours}:${minutes}:${secsPart.toFixed(3)}`;\n    }\n}\nconst KiB = 1024;\nconst MiB = 1024 * KiB;\nconst GiB = 1024 * MiB;\nconst TiB = 1024 * GiB;\nfunction fmtBytes(bytes) {\n    if (bytes > TiB) {\n        return `${(bytes / TiB).toFixed(3)} TiB`;\n    }\n    else if (bytes > GiB) {\n        return `${(bytes / GiB).toFixed(3)} GiB`;\n    }\n    else if (bytes > MiB) {\n        return `${(bytes / MiB).toFixed(1)} MiB`;\n    }\n    else if (bytes > KiB) {\n        return `${(bytes / KiB).toFixed(0)} KiB`;\n    }\n    else {\n        return `${bytes} B`;\n    }\n}\n\n\n//# sourceURL=webpack://ceph-to-zfs-webif/./src/ts/formatters.ts?");

/***/ }),

/***/ "./src/ts/main.ts":
/*!************************!*\
  !*** ./src/ts/main.ts ***!
  \************************/
/***/ ((__unused_webpack_module, exports, __webpack_require__) => {

eval("\nObject.defineProperty(exports, \"__esModule\", ({ value: true }));\nconst chooser_1 = __webpack_require__(/*! ./chooser */ \"./src/ts/chooser.ts\");\nconst util_1 = __webpack_require__(/*! ./util */ \"./src/ts/util.ts\");\nconst formatters_1 = __webpack_require__(/*! ./formatters */ \"./src/ts/formatters.ts\");\nclass JobItem {\n    _ele;\n    leftLabel;\n    rightLabel;\n    _prevLabel = null;\n    _prevStatusType = null;\n    constructor() {\n        this._ele = document.createElement(\"div\");\n        this.leftLabel = (0, util_1.el)('span');\n        this.rightLabel = (0, util_1.el)('span');\n        const leftDiv = (0, util_1.el)('div', {\n            classes: ['left-area'],\n            children: [this.leftLabel],\n        });\n        const rightDiv = (0, util_1.el)('div', {\n            classes: ['right-area'],\n            children: [this.rightLabel],\n        });\n        this._ele.replaceChildren(leftDiv, rightDiv);\n        this._ele.classList.add('chooser-item-bisected');\n    }\n    get element() {\n        return this._ele;\n    }\n    formatFor(key, value) {\n        if (this._prevLabel !== value.label) {\n            this.leftLabel.textContent = value.label;\n            this.leftLabel.title = value.label;\n            this._prevLabel = value.label;\n        }\n        const status = value.status;\n        // console.log(`formatFor ${key}`, status);\n        if (this._prevStatusType !== status.type) {\n            (0, util_1.addStatusLabelClasses)(this.rightLabel, status);\n            this._prevStatusType = status.type;\n            this.rightLabel.textContent = status.type;\n        }\n    }\n    setSelected(selected) {\n        if (selected) {\n            this._ele.classList.add('selected');\n        }\n        else {\n            this._ele.classList.remove('selected');\n        }\n    }\n}\nclass JobChooser extends chooser_1.Chooser {\n    extractKey(item) {\n        return item.id;\n    }\n    makeListItemUi() {\n        return new JobItem();\n    }\n}\nfunction fillTable(tbl, data) {\n    const rows = [];\n    function addRow(key, name, formatter) {\n        const value = data[key];\n        if (value !== undefined) {\n            rows.push((0, util_1.el)('tr', {\n                children: [\n                    (0, util_1.el)('td', { children: [name] }),\n                    (0, util_1.el)('td', { children: [formatter(value)] }),\n                ],\n            }));\n        }\n    }\n    addRow('snapName', \"Snapshot Name\", s => s);\n    addRow('cron', \"Cron\", s => s);\n    addRow('bytesWritten', \"Bytes Written\", formatters_1.fmtBytes);\n    addRow('bytesTrimmed', \"Bytes Trimmed\", formatters_1.fmtBytes);\n    addRow('prepStartTime', \"Prep Start\", formatters_1.fmtUnix);\n    addRow('prepEndTime', \"Prep End\", formatters_1.fmtUnix);\n    addRow('prepTime', \"Prep Time\", formatters_1.fmtDur);\n    addRow('runStartTime', \"Run Start\", formatters_1.fmtUnix);\n    addRow('runEndTime', \"Run End\", formatters_1.fmtUnix);\n    addRow('runTime', \"Run Time\", formatters_1.fmtDur);\n    tbl.replaceChildren(...rows);\n}\nclass JobStatusDisplay {\n    fullArea;\n    headerArea;\n    contentArea;\n    detailsTable;\n    snapshotsTable;\n    snapshotsTableBody;\n    // Lets us check whether this is a refresh of the same job or a new job\n    lastJobPath = null;\n    constructor() {\n        this.detailsTable = (0, util_1.el)('table', { classes: ['job-detail-table', 'extra-detail-table-area'] });\n        this.snapshotsTable = (0, util_1.el)('table', { classes: ['job-detail-table', 'snapshots-detail-table'] });\n        this.snapshotsTableBody = this.snapshotsTable.createTBody();\n        this.snapshotsTable.createTHead().replaceChildren((0, util_1.el)('tr', {\n            children: [\"Snapshot\", \"Source\", \"Receiver\"].map(t => (0, util_1.el)('th', { children: [t] })),\n        }));\n        this.snapshotsTable.style.display = 'none';\n        this.headerArea = (0, util_1.el)('div', { classes: ['header-area'] });\n        this.contentArea = (0, util_1.el)('div', { classes: ['content-area'] });\n        this.fullArea = (0, util_1.el)('div', {\n            classes: ['detail-display'],\n            children: [this.headerArea, this.contentArea],\n        });\n    }\n    setContent(job) {\n        if (job === null) {\n            this.headerArea.textContent = 'Job Status';\n            const nothingSelectedSpan = (0, util_1.el)(\"span\");\n            nothingSelectedSpan.textContent = 'No job selected';\n            this.contentArea.replaceChildren(nothingSelectedSpan);\n            return;\n        }\n        if (!(0, util_1.arraysEqual)(job.path, this.lastJobPath)) {\n            if (job.label !== job.id) {\n                this.headerArea.textContent = `${job.label} (${job.id})`;\n            }\n            else {\n                this.headerArea.textContent = job.label;\n            }\n            this.snapshotsTable.style.display = 'none';\n            this.lastJobPath = job.path;\n        }\n        if (Object.keys(job.extraData).length > 0) {\n            fillTable(this.detailsTable, job.extraData);\n            this.detailsTable.style.display = '';\n        }\n        else {\n            this.detailsTable.style.display = 'none';\n        }\n        const statusLabelSpan = (0, util_1.el)(\"span\");\n        const status = job.status;\n        statusLabelSpan.innerText = status.type + \": \";\n        (0, util_1.addStatusLabelClasses)(statusLabelSpan, status);\n        const statusDetailSpan = (0, util_1.el)(\"span\");\n        statusDetailSpan.innerText = status.message;\n        this.contentArea.replaceChildren(statusLabelSpan, statusDetailSpan, this.detailsTable, this.snapshotsTable);\n        const parts = job.path.join(\"/\");\n        fetch(makeUrl(`/api/taskdetails/${parts}`)).then(response => response.json())\n            .then(data => {\n            // Don't try to refresh if another item has already been selected\n            if (!(0, util_1.arraysEqual)(job.path, this.lastJobPath)) {\n                return;\n            }\n            const d = data;\n            const snapshotReport = d.detailData.snapshotReport;\n            const out = [];\n            if (snapshotReport !== undefined) {\n                snapshotReport.snapshots.forEach(snap => {\n                    const nameCell = (0, util_1.el)('td', { children: [snap.name] });\n                    const srcCell = makeSnapshotCell(snap.source);\n                    const rcvCell = makeSnapshotCell(snap.receiver);\n                    out.push((0, util_1.el)('tr', { children: [nameCell, srcCell, rcvCell] }));\n                });\n                this.snapshotsTableBody.replaceChildren(...out);\n                this.snapshotsTable.style.display = '';\n            }\n            else {\n                this.snapshotsTable.style.display = 'none';\n            }\n        });\n    }\n}\nfunction makeSnapshotCell(snap) {\n    if (snap === null) {\n        return (0, util_1.el)('td', {\n            children: ['Absent'],\n            classes: ['snapshot-absent'],\n        });\n    }\n    else {\n        if (snap.pruned) {\n            return (0, util_1.el)('td', {\n                children: ['Pruned'],\n                classes: ['snapshot-pruned'],\n            });\n        }\n        else {\n            return (0, util_1.el)('td', {\n                children: ['Kept'],\n                classes: ['snapshot-kept'],\n            });\n        }\n    }\n}\nconst baseUrl = function () {\n    const ovr = localStorage.getItem(\"apiServerOverride\");\n    if (ovr !== null) {\n        try {\n            return new URL(ovr);\n        }\n        catch (e) {\n            console.error(`Invalid override: '${ovr}'`);\n        }\n    }\n    return new URL(document.location.href);\n}();\nfunction makeUrl(urlPart) {\n    return new URL(urlPart, baseUrl);\n}\nfunction btn(label, listener, tooltip) {\n    const out = document.createElement(\"button\");\n    out.textContent = label;\n    out.addEventListener(\"click\", listener);\n    if (tooltip) {\n        out.title = tooltip;\n    }\n    return out;\n}\nclass Toolbar {\n    element;\n    timeDisplay;\n    constructor(refreshHook) {\n        const out = document.createElement(\"div\");\n        out.classList.add('toolbar');\n        const refreshButton = btn(\"Refresh\", refreshHook);\n        // const prepAllButton = btn(\"Prep All\", () => fetch(makeUrl(\"/api/prepall\")));\n        const runAllButton = btn(\"Run All\", () => fetch(makeUrl(\"/api/startall\")));\n        this.timeDisplay = (0, util_1.el)('span');\n        this.timeDisplay.classList.add(\"time-display\");\n        this.timeDisplay.textContent = 'Connecting...';\n        const leftSection = (0, util_1.el)('div', {\n            classes: ['toolbar-left'],\n            children: [refreshButton, runAllButton],\n        });\n        const midSpacer = (0, util_1.el)('div', { classes: ['toolbar-mid-spacer'] });\n        const rightSection = (0, util_1.el)('div', {\n            classes: ['toolbar-right'],\n            children: [this.timeDisplay],\n        });\n        out.replaceChildren(leftSection, midSpacer, rightSection);\n        this.element = out;\n    }\n    setTimestamp(unixSecs) {\n        this.timeDisplay.textContent = (0, formatters_1.fmtUnix)(unixSecs);\n    }\n}\nclass MainUi {\n    mainTable;\n    jobChooser;\n    imageChooser;\n    taskDetailHolder;\n    toolbar;\n    outer;\n    constructor() {\n        const mainDiv = document.createElement(\"div\");\n        mainDiv.classList.add(\"main-table\");\n        this.mainTable = mainDiv;\n        const jobStatusDisplay = new JobStatusDisplay();\n        this.taskDetailHolder = jobStatusDisplay.fullArea;\n        // Flows:\n        // User explicitly clicks an image: Select the image, keep the pool selected, display image details\n        // User explicitly clicks a pool: Select the pool, deselect the image, display pool details\n        // User refreshes with pool selected: Keep the pool selected, display new pool details\n        // user refreshes with image selected: Keep the pool+image selected, display new image details\n        this.imageChooser = new JobChooser(['Images'], s => {\n            const item = s.new?.item;\n            if (item !== undefined) {\n                jobStatusDisplay.setContent(item);\n            }\n            return true;\n        });\n        this.jobChooser = new JobChooser(['Jobs'], (s, e) => {\n            const item = s.new?.item;\n            this.imageChooser.allValues = item?.children ?? [];\n            // if (this.imageChooser.hasInitialData) {\n            //     this.imageChooser.setSelection(null);\n            // }\n            if (e) {\n                this.imageChooser.setSelection(null, false);\n                jobStatusDisplay.setContent(item ?? null);\n            }\n            else if (this.jobChooser.allValues.length === 0) {\n                jobStatusDisplay.setContent(null);\n            }\n            else if (item !== undefined && this.imageChooser.selectedItem === null) {\n                jobStatusDisplay.setContent(item);\n            }\n            return true;\n        });\n        mainDiv.replaceChildren(this.jobChooser.fullElement, this.imageChooser.fullElement, this.taskDetailHolder);\n        this.toolbar = new Toolbar(() => this.refresh());\n        this.outer = document.createElement(\"div\");\n        this.outer.classList.add(\"main-page\");\n        this.outer.replaceChildren(this.toolbar.element, this.mainTable);\n    }\n    get mainElement() {\n        return this.outer;\n    }\n    async refresh() {\n        const allTaskUrl = new URL(\"/api/alltasks\", baseUrl);\n        const response = await fetch(allTaskUrl).then(response => response.json());\n        const mainJobRaw = response.task;\n        const mainJob = {\n            ...mainJobRaw,\n            path: [],\n            children: mainJobRaw.children.map(jr => addPath(jr, [])),\n        };\n        this.setTimeStamp(response.serverInfo.unixTime);\n        this.jobChooser.allValues = mainJob.children;\n    }\n    setTimeStamp(unixSeconds) {\n        this.toolbar.setTimestamp(unixSeconds);\n    }\n}\nfunction addPath(jobRaw, parentPath) {\n    const path = [...parentPath, jobRaw.id];\n    return {\n        ...jobRaw,\n        path: path,\n        children: jobRaw.children.map(jr => addPath(jr, path)),\n    };\n}\ndocument.addEventListener('DOMContentLoaded', () => {\n    const main = new MainUi();\n    document.querySelector('body')?.append(main.mainElement);\n    // For debugging\n    // @ts-expect-error debugging\n    window['mainUI'] = main;\n    async function refreshLoop() {\n        try {\n            // @ts-expect-error debugging\n            if (!window['pause']) {\n                await main.refresh();\n            }\n        }\n        catch (e) {\n            console.error(\"Error refreshing\", e);\n        }\n        finally {\n            setTimeout(refreshLoop, 1_000);\n        }\n    }\n    // noinspection JSIgnoredPromiseFromCall\n    refreshLoop();\n});\n\n\n//# sourceURL=webpack://ceph-to-zfs-webif/./src/ts/main.ts?");

/***/ }),

/***/ "./src/ts/util.ts":
/*!************************!*\
  !*** ./src/ts/util.ts ***!
  \************************/
/***/ ((__unused_webpack_module, exports) => {

eval("\nObject.defineProperty(exports, \"__esModule\", ({ value: true }));\nexports.el = el;\nexports.addStatusLabelClasses = addStatusLabelClasses;\nexports.arraysEqual = arraysEqual;\nfunction el(tagName, opts) {\n    const out = document.createElement(tagName);\n    if (opts) {\n        if (opts.classes) {\n            out.classList.add(...opts.classes);\n        }\n        if (opts.children) {\n            out.replaceChildren(...opts.children);\n        }\n    }\n    return out;\n}\nfunction addStatusLabelClasses(element, status) {\n    element.classList.add('status-label');\n    element.classList.remove('terminal-failed', 'terminal-success', 'in-progress', 'not-started');\n    if (status.isTerminal) {\n        if (status.isBad) {\n            element.classList.add('terminal-failed');\n        }\n        else {\n            element.classList.add('terminal-success');\n        }\n    }\n    else if (status.isActive) {\n        element.classList.add('in-progress');\n    }\n    else {\n        element.classList.add('not-started');\n    }\n}\nfunction arraysEqual(left, right) {\n    // Both null\n    if (left === null && right === null) {\n        return true;\n    }\n    // One null, either not null (both null handled above)\n    if (left === null || right === null) {\n        return false;\n    }\n    if (left.length !== right.length) {\n        return false;\n    }\n    for (let i = 0; i < left.length; i++) {\n        if (left[i] !== right[i]) {\n            return false;\n        }\n    }\n    return true;\n}\n\n\n//# sourceURL=webpack://ceph-to-zfs-webif/./src/ts/util.ts?");

/***/ })

/******/ 	});
/************************************************************************/
/******/ 	// The module cache
/******/ 	var __webpack_module_cache__ = {};
/******/ 	
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/ 		// Check if module is in cache
/******/ 		var cachedModule = __webpack_module_cache__[moduleId];
/******/ 		if (cachedModule !== undefined) {
/******/ 			return cachedModule.exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = __webpack_module_cache__[moduleId] = {
/******/ 			// no module.id needed
/******/ 			// no module.loaded needed
/******/ 			exports: {}
/******/ 		};
/******/ 	
/******/ 		// Execute the module function
/******/ 		__webpack_modules__[moduleId](module, module.exports, __webpack_require__);
/******/ 	
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/ 	
/************************************************************************/
/******/ 	
/******/ 	// startup
/******/ 	// Load entry module and return exports
/******/ 	// This entry module can't be inlined because the eval devtool is used.
/******/ 	var __webpack_exports__ = __webpack_require__("./src/ts/main.ts");
/******/ 	
/******/ })()
;