export namespace main {
	
	export class AppInfo {
	    version: string;
	    mode: string;
	    ready: boolean;
	    startError: string;
	    terminal: string;
	    serverUrl: string;
	    port: number;
	    addresses: string[];
	    dataDir: string;
	    invoiceCode: string;
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
	        this.serverUrl = source["serverUrl"];
	        this.port = source["port"];
	        this.addresses = source["addresses"];
	        this.dataDir = source["dataDir"];
	        this.invoiceCode = source["invoiceCode"];
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

}

export namespace model {
	
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
	export class ReportQuery {
	    period: string;
	    from: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new ReportQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.from = source["from"];
	        this.to = source["to"];
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
	    total: number;
	    paid: number;
	    changeAmount: number;
	    createdAt: string;
	    printCount: number;
	    items: SaleItem[];
	    deletedAt?: string;
	    deletedByName: string;
	    deleteReason: string;
	
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
	        this.total = source["total"];
	        this.paid = source["paid"];
	        this.changeAmount = source["changeAmount"];
	        this.createdAt = source["createdAt"];
	        this.printCount = source["printCount"];
	        this.items = this.convertValues(source["items"], SaleItem);
	        this.deletedAt = source["deletedAt"];
	        this.deletedByName = source["deletedByName"];
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
	export class SaleQuery {
	    from: string;
	    to: string;
	    deleted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaleQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.deleted = source["deleted"];
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
	    }
	}

}

