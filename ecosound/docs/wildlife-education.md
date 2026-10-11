# Wildlife education and sources

Species explanations are retrieval-grounded. `/api/species/{scientificName}` looks up an exact scientific-name match in `species_knowledge`; `/api/discoveries/{id}/explain` resolves the immutable BirdNET prediction first, then asks local Qwen to summarize only the curated row. Model-unavailable and missing-source responses do not fall back to the LLM's general knowledge. Responses include the curated facts, source URL, source license, ethical guidance, and an AI-content disclaimer.

The initial curated entry is Hadada Ibis (*Bostrychia hagedash*). Its ecological role is explicitly labeled an ecological summary; its conservation note identifies the date of the referenced IUCN assessment and does not present it as a current reassessment. The source is the [IUCN Red List assessment](https://doi.org/10.2305/IUCN.UK.2016-3.RLTS.T22697463A93614797.en). The record is paraphrased, carries the source URL/license notice, and is not a bulk import. No external GBIF/eBird dataset has been imported. Add future entries only after reviewing terms, attribution, access restrictions, update date, and sensitive-species location policy.

The interface encourages returning attention to the outdoors. Qwen does not create an independent reference and cannot change prediction or confidence fields. A user confirmation remains personal, not scientific verification.
