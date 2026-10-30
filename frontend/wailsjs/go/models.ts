export namespace main {
	
	export class AppInfo {
	    version: string;
	    mode: string;
	    ready: boolean;
	    startError: string;
	    terminal: string;
	    deviceId: string;
	    deviceName: string;
	    serverUrl: string;
	    port: number;
	    addresses: string[];
	    dataDir: string;
	    printerName: string;
	    paperWidth: number;
	    os: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.mode = source["mode"];
	        this.ready = source["ready"];
	        this.startError = source["startError"];
	        this.terminal = source["terminal"];
	        this.deviceId = source["deviceId"];
	        this.deviceName = source["deviceName"];
	        this.serverUrl = source["serverUrl"];
	        this.port = source["port"];
	        this.addresses = source["addresses"];
	        this.dataDir = source["dataDir"];
	        this.printerName = source["printerName"];
	        this.paperWidth = source["paperWidth"];
	        this.os = source["os"];
	    }
	}
	export class ReceiptView {
	    lines: receipt.Line[];
	    logo: string;
	    printCount: number;
	    deleted: boolean;
	    deleteReason: string;
	
	    static createFrom(source: any = {}) {
	        return new ReceiptView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = this.convertValues(source["lines"], receipt.Line);
	        this.logo = source["logo"];
	        this.printCount = source["printCount"];
	        this.deleted = source["deleted"];
	        this.deleteReason = source["deleteReason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RestoreInfo {
	    path: string;
	    name: string;
	    size: number;
	    sales: number;
	    lastSale: string;
	    users: number;
	
	    static createFrom(source: any = {}) {
	        return new RestoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sales = source["sales"];
	        this.lastSale = source["lastSale"];
	        this.users = source["users"];
	    }
	}

}

export namespace model {
	
	export class CloseShiftInput {
	    shiftId: number;
	    revision: number;
	    actualCash: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new CloseShiftInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shiftId = source["shiftId"];
	        this.revision = source["revision"];
	        this.actualCash = source["actualCash"];
	        this.note = source["note"];
	    }
	}
	export class Counter {
	    id: number;
	    name: string;
	    invoiceCode: string;
	    active: boolean;
	    deviceId: string;
	    deviceName: string;
	    lastSeenAt?: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Counter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.invoiceCode = source["invoiceCode"];
	        this.active = source["active"];
	        this.deviceId = source["deviceId"];
	        this.deviceName = source["deviceName"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class CounterInput {
	    id: number;
	    name: string;
	    invoiceCode: string;
	
	    static createFrom(source: any = {}) {
	        return new CounterInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.invoiceCode = source["invoiceCode"];
	    }
	}
	export class DeletedSale {
	    id: number;
	    invoiceNo: string;
	    total: number;
	    createdAt: string;
	    deletedAt: string;
	    deletedByName: string;
	    reason: string;
	    replacedByInvoiceNo: string;
	
	    static createFrom(source: any = {}) {
	        return new DeletedSale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.invoiceNo = source["invoiceNo"];
	        this.total = source["total"];
	        this.createdAt = source["createdAt"];
	        this.deletedAt = source["deletedAt"];
	        this.deletedByName = source["deletedByName"];
	        this.reason = source["reason"];
	        this.replacedByInvoiceNo = source["replacedByInvoiceNo"];
	    }
	}
	export class ReportTicketRow {
	    ticketId: number;
	    ticketName: string;
	    category: string;
	    qty: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ReportTicketRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ticketId = source["ticketId"];
	        this.ticketName = source["ticketName"];
	        this.category = source["category"];
	        this.qty = source["qty"];
	        this.total = source["total"];
	    }
	}
	export class ReportRow {
	    period: string;
	    transactions: number;
	    qty: number;
	    total: number;
	    ticketQty: number;
	    ticketTotal: number;
	    topupQty: number;
	    topupTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new ReportRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.transactions = source["transactions"];
	        this.qty = source["qty"];
	        this.total = source["total"];
	        this.ticketQty = source["ticketQty"];
	        this.ticketTotal = source["ticketTotal"];
	        this.topupQty = source["topupQty"];
	        this.topupTotal = source["topupTotal"];
	    }
	}
	export class Report {
	    period: string;
	    from: string;
	    to: string;
	    rows: ReportRow[];
	    tickets: ReportTicketRow[];
	    transactions: number;
	    qty: number;
	    total: number;
	    ticketQty: number;
	    ticketTotal: number;
	    topupQty: number;
	    topupTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.rows = this.convertValues(source["rows"], ReportRow);
	        this.tickets = this.convertValues(source["tickets"], ReportTicketRow);
	        this.transactions = source["transactions"];
	        this.qty = source["qty"];
	        this.total = source["total"];
	        this.ticketQty = source["ticketQty"];
	        this.ticketTotal = source["ticketTotal"];
	        this.topupQty = source["topupQty"];
	        this.topupTotal = source["topupTotal"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReportCashier {
	    id: number;
	    fullName: string;
	    deleted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReportCashier(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fullName = source["fullName"];
	        this.deleted = source["deleted"];
	    }
	}
	export class ShiftType {
	    id: number;
	    name: string;
	    startTime: string;
	    endTime: string;
	    active: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ShiftType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.active = source["active"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ReportFilters {
	    cashiers: ReportCashier[];
	    counters: Counter[];
	    shiftTypes: ShiftType[];
	
	    static createFrom(source: any = {}) {
	        return new ReportFilters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cashiers = this.convertValues(source["cashiers"], ReportCashier);
	        this.counters = this.convertValues(source["counters"], Counter);
	        this.shiftTypes = this.convertValues(source["shiftTypes"], ShiftType);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReportQuery {
	    period: string;
	    from: string;
	    to: string;
	    category: string;
	    userId: number;
	    counterId: number;
	    shiftTypeId: number;
	
	    static createFrom(source: any = {}) {
	        return new ReportQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.category = source["category"];
	        this.userId = source["userId"];
	        this.counterId = source["counterId"];
	        this.shiftTypeId = source["shiftTypeId"];
	    }
	}
	
	
	export class SaleItem {
	    id: number;
	    ticketId: number;
	    ticketName: string;
	    category: string;
	    price: number;
	    qty: number;
	    subtotal: number;
	
	    static createFrom(source: any = {}) {
	        return new SaleItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ticketId = source["ticketId"];
	        this.ticketName = source["ticketName"];
	        this.category = source["category"];
	        this.price = source["price"];
	        this.qty = source["qty"];
	        this.subtotal = source["subtotal"];
	    }
	}
	export class Sale {
	    id: number;
	    invoiceNo: string;
	    userId: number;
	    cashierName: string;
	    terminal: string;
	    counterId: number;
	    counterName: string;
	    shiftName: string;
	    total: number;
	    paid: number;
	    changeAmount: number;
	    createdAt: string;
	    printCount: number;
	    items: SaleItem[];
	    deletedAt?: string;
	    deletedByName: string;
	    deleteReason: string;
	    shiftId: number;
	    replacesSaleId: number;
	    replacesInvoiceNo: string;
	    replacedByInvoiceNo: string;
	
	    static createFrom(source: any = {}) {
	        return new Sale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.invoiceNo = source["invoiceNo"];
	        this.userId = source["userId"];
	        this.cashierName = source["cashierName"];
	        this.terminal = source["terminal"];
	        this.counterId = source["counterId"];
	        this.counterName = source["counterName"];
	        this.shiftName = source["shiftName"];
	        this.total = source["total"];
	        this.paid = source["paid"];
	        this.changeAmount = source["changeAmount"];
	        this.createdAt = source["createdAt"];
	        this.printCount = source["printCount"];
	        this.items = this.convertValues(source["items"], SaleItem);
	        this.deletedAt = source["deletedAt"];
	        this.deletedByName = source["deletedByName"];
	        this.deleteReason = source["deleteReason"];
	        this.shiftId = source["shiftId"];
	        this.replacesSaleId = source["replacesSaleId"];
	        this.replacesInvoiceNo = source["replacesInvoiceNo"];
	        this.replacedByInvoiceNo = source["replacedByInvoiceNo"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SaleItemInput {
	    ticketId: number;
	    qty: number;
	
	    static createFrom(source: any = {}) {
	        return new SaleItemInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ticketId = source["ticketId"];
	        this.qty = source["qty"];
	    }
	}
	export class SalePage {
	    sales: Sale[];
	    count: number;
	    amount: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new SalePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sales = this.convertValues(source["sales"], Sale);
	        this.count = source["count"];
	        this.amount = source["amount"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SalePageQuery {
	    from: string;
	    to: string;
	    deleted: boolean;
	    search: string;
	    category: string;
	    userId: number;
	    counterId: number;
	    shiftTypeId: number;
	    page: number;
	    pageSize: number;
	    sort: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new SalePageQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.deleted = source["deleted"];
	        this.search = source["search"];
	        this.category = source["category"];
	        this.userId = source["userId"];
	        this.counterId = source["counterId"];
	        this.shiftTypeId = source["shiftTypeId"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.sort = source["sort"];
	        this.dir = source["dir"];
	    }
	}
	export class SaleQuery {
	    from: string;
	    to: string;
	    deleted: boolean;
	    search: string;
	    category: string;
	    userId: number;
	    counterId: number;
	    shiftTypeId: number;
	
	    static createFrom(source: any = {}) {
	        return new SaleQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.deleted = source["deleted"];
	        this.search = source["search"];
	        this.category = source["category"];
	        this.userId = source["userId"];
	        this.counterId = source["counterId"];
	        this.shiftTypeId = source["shiftTypeId"];
	    }
	}
	export class Settings {
	    businessName: string;
	    address: string;
	    footer: string;
	    invoicePrefix: string;
	    purposeWeekday: string;
	    purposeWeekend: string;
	    hasLogo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.businessName = source["businessName"];
	        this.address = source["address"];
	        this.footer = source["footer"];
	        this.invoicePrefix = source["invoicePrefix"];
	        this.purposeWeekday = source["purposeWeekday"];
	        this.purposeWeekend = source["purposeWeekend"];
	        this.hasLogo = source["hasLogo"];
	    }
	}
	export class Shift {
	    id: number;
	    typeId: number;
	    name: string;
	    startTime: string;
	    endTime: string;
	    businessDate: string;
	    scheduledEndAt: string;
	    userId: number;
	    cashierName: string;
	    counterId: number;
	    counterName: string;
	    openingCash: number;
	    openedAt: string;
	    closedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new Shift(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.typeId = source["typeId"];
	        this.name = source["name"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.businessDate = source["businessDate"];
	        this.scheduledEndAt = source["scheduledEndAt"];
	        this.userId = source["userId"];
	        this.cashierName = source["cashierName"];
	        this.counterId = source["counterId"];
	        this.counterName = source["counterName"];
	        this.openingCash = source["openingCash"];
	        this.openedAt = source["openedAt"];
	        this.closedAt = source["closedAt"];
	    }
	}
	export class ShiftSales {
	    transactions: number;
	    ticketQty: number;
	    ticketTotal: number;
	    topupQty: number;
	    topupTotal: number;
	    total: number;
	    firstSaleAt: string;
	    lastSaleAt: string;
	    deletedCount: number;
	    deletedTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new ShiftSales(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transactions = source["transactions"];
	        this.ticketQty = source["ticketQty"];
	        this.ticketTotal = source["ticketTotal"];
	        this.topupQty = source["topupQty"];
	        this.topupTotal = source["topupTotal"];
	        this.total = source["total"];
	        this.firstSaleAt = source["firstSaleAt"];
	        this.lastSaleAt = source["lastSaleAt"];
	        this.deletedCount = source["deletedCount"];
	        this.deletedTotal = source["deletedTotal"];
	    }
	}
	export class ShiftClosing {
	    id: number;
	    shift: Shift;
	    revision: number;
	    sales: ShiftSales;
	    expectedCash: number;
	    actualCash: number;
	    difference: number;
	    note: string;
	    closedByName: string;
	    closedAt: string;
	    printCount: number;
	    previousActual?: number;
	    deleted: DeletedSale[];
	
	    static createFrom(source: any = {}) {
	        return new ShiftClosing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.shift = this.convertValues(source["shift"], Shift);
	        this.revision = source["revision"];
	        this.sales = this.convertValues(source["sales"], ShiftSales);
	        this.expectedCash = source["expectedCash"];
	        this.actualCash = source["actualCash"];
	        this.difference = source["difference"];
	        this.note = source["note"];
	        this.closedByName = source["closedByName"];
	        this.closedAt = source["closedAt"];
	        this.printCount = source["printCount"];
	        this.previousActual = source["previousActual"];
	        this.deleted = this.convertValues(source["deleted"], DeletedSale);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ShiftReport {
	    shift: Shift;
	    sales: ShiftSales;
	    expectedCash: number;
	    deleted: DeletedSale[];
	    closing?: ShiftClosing;
	    history: ShiftClosing[];
	    changed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShiftReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shift = this.convertValues(source["shift"], Shift);
	        this.sales = this.convertValues(source["sales"], ShiftSales);
	        this.expectedCash = source["expectedCash"];
	        this.deleted = this.convertValues(source["deleted"], DeletedSale);
	        this.closing = this.convertValues(source["closing"], ShiftClosing);
	        this.history = this.convertValues(source["history"], ShiftClosing);
	        this.changed = source["changed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ShiftDay {
	    date: string;
	    shifts: ShiftReport[];
	    unclosed: Shift[];
	
	    static createFrom(source: any = {}) {
	        return new ShiftDay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.shifts = this.convertValues(source["shifts"], ShiftReport);
	        this.unclosed = this.convertValues(source["unclosed"], Shift);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ShiftStatus {
	    counterId: number;
	    counterName: string;
	    own?: ShiftReport;
	    blocking?: Shift;
	    types: ShiftType[];
	    suggestedTypeId: number;
	    serverTime: string;
	
	    static createFrom(source: any = {}) {
	        return new ShiftStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.counterId = source["counterId"];
	        this.counterName = source["counterName"];
	        this.own = this.convertValues(source["own"], ShiftReport);
	        this.blocking = this.convertValues(source["blocking"], Shift);
	        this.types = this.convertValues(source["types"], ShiftType);
	        this.suggestedTypeId = source["suggestedTypeId"];
	        this.serverTime = source["serverTime"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ShiftTypeInput {
	    id: number;
	    name: string;
	    startTime: string;
	    endTime: string;
	
	    static createFrom(source: any = {}) {
	        return new ShiftTypeInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	    }
	}
	export class Status {
	    version: string;
	    hasUsers: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.hasUsers = source["hasUsers"];
	    }
	}
	export class Ticket {
	    id: number;
	    name: string;
	    price: number;
	    category: string;
	    color: string;
	    active: boolean;
	    createdAt: string;
	    updatedAt: string;
	    deletedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new Ticket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.category = source["category"];
	        this.color = source["color"];
	        this.active = source["active"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.deletedAt = source["deletedAt"];
	    }
	}
	export class TicketInput {
	    id: number;
	    name: string;
	    price: number;
	    category: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new TicketInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.category = source["category"];
	        this.color = source["color"];
	    }
	}
	export class TicketPage {
	    tickets: Ticket[];
	    count: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new TicketPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tickets = this.convertValues(source["tickets"], Ticket);
	        this.count = source["count"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TicketQuery {
	    search: string;
	    category: string;
	    status: string;
	    page: number;
	    pageSize: number;
	    sort: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new TicketQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.search = source["search"];
	        this.category = source["category"];
	        this.status = source["status"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.sort = source["sort"];
	        this.dir = source["dir"];
	    }
	}
	export class User {
	    id: number;
	    username: string;
	    fullName: string;
	    role: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.fullName = source["fullName"];
	        this.role = source["role"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class UserInput {
	    id: number;
	    username: string;
	    fullName: string;
	    role: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new UserInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.fullName = source["fullName"];
	        this.role = source["role"];
	        this.password = source["password"];
	    }
	}

}

export namespace receipt {
	
	export class Line {
	    text: string;
	    bold: boolean;
	    size: string;
	    logo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Line(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.bold = source["bold"];
	        this.size = source["size"];
	        this.logo = source["logo"];
	    }
	}

}

export namespace updater {
	
	export class Info {
	    configured: boolean;
	    available: boolean;
	    current: string;
	    latest: string;
	    notes: string;
	    mandatory: boolean;
	    fromServer: boolean;
	    manual: boolean;
	    serverBehind: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.available = source["available"];
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.notes = source["notes"];
	        this.mandatory = source["mandatory"];
	        this.fromServer = source["fromServer"];
	        this.manual = source["manual"];
	        this.serverBehind = source["serverBehind"];
	    }
	}

}

