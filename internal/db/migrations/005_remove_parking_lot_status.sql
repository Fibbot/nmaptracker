BEGIN TRANSACTION;

UPDATE port
   SET work_status = 'flagged'
 WHERE work_status = 'parking_lot';

COMMIT;
