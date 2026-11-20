export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

export async function api<T = unknown>(path: string, init: RequestInit & { json?: unknown } = {}): Promise<T> {
	const { json, headers, ...rest } = init;
	const res = await fetch('/api/v1' + path, {
		credentials: 'include',
		...rest,
		headers: { ...(json !== undefined ? { 'Content-Type': 'application/json' } : {}), ...headers },
		body: json !== undefined ? JSON.stringify(json) : rest.body
	});
	if (res.status === 204) return undefined as T;
	const data = await res.json().catch(() => ({}));
	if (!res.ok) throw new ApiError(res.status, data.error ?? 'Terjadi kesalahan');
	return data as T;
}

export type Me = { name: string; points: number; phone: string; phone_verified: boolean; otp_available: boolean };
export type Reward = { id: number; name: string; cost_points: number; stock: number; image_url: string | null };
export type Redemption = {
	id: number;
	reward: string;
	cost: number;
	status: 'pending' | 'fulfilled' | 'cancelled';
	code: string;
	created_at: string;
};
export type Admin ={ id: number; email: string; role: string };
export type Product = {
	id: number;
	sku: string;
	name: string;
	points_per_unit: number;
	active: boolean;
};
export type Batch = {
	id: number;
	product_id: number;
	product_name: string;
	label: string;
	qty: number;
	status: 'generated' | 'printed' | 'active';
	created_at: string;
	printed_at: string | null;
	counts: Partial<Record<'generated' | 'active' | 'claimed' | 'void', number>>;
};
export type AdminReward = { id: number; name: string; cost_points: number; stock: number; image_url: string | null; active: boolean };
export type AdminRedemption = {
	id: number;
	code: string;
	status: 'pending' | 'fulfilled' | 'cancelled';
	reward: string;
	cost: number;
	customer: string;
	phone: string;
	created_at: string;
	fulfilled_at: string | null;
};
export type AdminCustomer = { id: number; name: string; phone: string; points: number; phone_verified: boolean; created_at: string };
export type Summary = {
	customers: number;
	verified_customers: number;
	codes_active: number;
	codes_claimed: number;
	points_issued: number;
	points_redeemed: number;
	redemptions_pending: number;
	redemptions_fulfilled: number;
};
export type AuditEntry = { id: number; admin: string | null; action: string; payload: string; created_at: string };
