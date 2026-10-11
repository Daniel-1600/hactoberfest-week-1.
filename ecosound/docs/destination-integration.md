# Destinations and conservancy experiences

Destinations are reusable records, not app code or generated claims. Published trails are returned only where `status='verified'`; an experience linked to a trail is returned only if its trail is verified. The schema supports GeoJSON for future authorized, curated trail geometry, but the current demo has no geometry or verified trail records.

The Explore screen features the seeded Karura-inspired **Karura Forest — Hidden Voices Adventure** as a route-free concept and explicitly says: **Concept demonstration — not affiliated with Karura Forest management.** The destination screen lets a visitor read the forest introduction and start a generic forest listening mission; it asks them to use current official visitor guidance without naming a route or claiming an integration. Its existence is a product concept, not an endorsement, partnership, or statement of access. The concept cannot be passed to the approved-destination generator. Future destination management requires a reviewed content workflow and authorized credentials; there are deliberately no public admin write endpoints.

Routes: `GET /api/destinations`, `/api/destinations/{id}`, `/api/destinations/{id}/trails`, and `/api/destinations/{id}/experiences`. Do not add unreviewed route geometry or imply accessibility/access guarantees from an LLM.
