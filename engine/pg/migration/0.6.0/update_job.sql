UPDATE
	job
SET
	bpmn_element_type = b.bpmn_element_type
FROM
	job a
INNER JOIN
	element_instance b
ON
	b.partition = a.partition AND
	b.id = a.element_instance_id
;
