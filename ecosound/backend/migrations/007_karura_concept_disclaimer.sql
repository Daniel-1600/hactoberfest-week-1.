UPDATE destinations
SET description = description || ' Concept demonstration — not affiliated with Karura Forest management.'
WHERE id='karura-concept' AND instr(description, 'not affiliated with Karura Forest management')=0;
