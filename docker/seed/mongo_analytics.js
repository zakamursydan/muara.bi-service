// Sample MongoDB source for the federated-query demo (PLAN.md section 4.4).
//
//   docker exec -i muara_sample_mongo mongosh \
//     -u mongo_user -p mongo_password --authenticationDatabase admin --quiet \
//     < docker/seed/mongo_analytics.js
//
// `sessions.order_id` is the join key against muara_sample_postgres
// (sample_ecommerce.orders). The nested `customer` document and the `tags`
// array are deliberate: the Go streaming adapter flattens the former into
// `customer_address_city` style columns and serialises the latter as JSON so
// the mounted DuckDB view keeps a stable, analyzable column set.

const sample = db.getSiblingDB("sample_analytics");

sample.sessions.drop();
sample.page_views.drop();

const sessions = [
  {
    session_id: "SES-1001",
    user_id: "USR-11",
    order_id: 1,
    device: "ios",
    campaign: "ramadan-2026",
    duration_seconds: 412,
    customer: { name: "Aisyah", address: { city: "Jakarta", province: "DKI Jakarta" } },
    tags: ["repeat-buyer", "loyalty"],
    created_at: ISODate("2026-10-01T08:15:00Z"),
  },
  {
    session_id: "SES-1002",
    user_id: "USR-12",
    order_id: 2,
    device: "android",
    campaign: "ramadan-2026",
    duration_seconds: 190,
    customer: { name: "Bilal", address: { city: "Bandung", province: "Jawa Barat" } },
    tags: ["first-order"],
    created_at: ISODate("2026-10-01T11:02:00Z"),
  },
  {
    session_id: "SES-1003",
    user_id: "USR-13",
    order_id: 3,
    device: "web",
    campaign: "weekend-push",
    duration_seconds: 655,
    customer: { name: "Chandra", address: { city: "Surabaya", province: "Jawa Timur" } },
    tags: ["repeat-buyer"],
    created_at: ISODate("2026-10-02T03:40:00Z"),
  },
  {
    session_id: "SES-1004",
    user_id: "USR-14",
    order_id: 4,
    device: "pos",
    campaign: null,
    duration_seconds: 75,
    customer: { name: "Dewi", address: { city: "Medan", province: "Sumatera Utara" } },
    tags: [],
    created_at: ISODate("2026-10-02T09:25:00Z"),
  },
  {
    session_id: "SES-1005",
    user_id: "USR-15",
    order_id: 5,
    device: "android",
    campaign: "weekend-push",
    duration_seconds: 320,
    customer: { name: "Fauzan", address: { city: "Makassar", province: "Sulawesi Selatan" } },
    tags: ["loyalty"],
    created_at: ISODate("2026-10-03T05:55:00Z"),
  },
  {
    session_id: "SES-1006",
    user_id: "USR-16",
    order_id: 6,
    device: "ios",
    campaign: "weekend-push",
    duration_seconds: 208,
    customer: { name: "Gita", address: { city: "Jakarta", province: "DKI Jakarta" } },
    tags: ["repeat-buyer", "loyalty"],
    created_at: ISODate("2026-10-03T12:30:00Z"),
  },
  {
    session_id: "SES-1007",
    user_id: "USR-17",
    order_id: 99,
    device: "web",
    campaign: "abandoned-cart",
    duration_seconds: 1450,
    customer: { name: "Hendra", address: { city: "Semarang", province: "Jawa Tengah" } },
    tags: ["no-conversion"],
    created_at: ISODate("2026-10-04T01:10:00Z"),
  },
];

const pageViews = [
  { view_id: "PV-1", session_id: "SES-1001", path: "/product/kopi-arabika", dwell_ms: 18000 },
  { view_id: "PV-2", session_id: "SES-1001", path: "/checkout", dwell_ms: 9000 },
  { view_id: "PV-3", session_id: "SES-1002", path: "/product/teh-matcha", dwell_ms: 12000 },
  { view_id: "PV-4", session_id: "SES-1003", path: "/product/kopi-arabika", dwell_ms: 41000 },
  { view_id: "PV-5", session_id: "SES-1004", path: "/store/brg-03", dwell_ms: 4000 },
  { view_id: "PV-6", session_id: "SES-1005", path: "/product/teh-matcha", dwell_ms: 22000 },
  { view_id: "PV-7", session_id: "SES-1006", path: "/product/susu-oat", dwell_ms: 15000 },
  { view_id: "PV-8", session_id: "SES-1007", path: "/cart", dwell_ms: 60000 },
];

sample.sessions.insertMany(sessions);
sample.page_views.insertMany(pageViews);

sample.sessions.createIndex({ order_id: 1 });
sample.page_views.createIndex({ session_id: 1 });

print(`sample_analytics seeded: ${sample.sessions.countDocuments()} sessions, ${sample.page_views.countDocuments()} page_views`);
