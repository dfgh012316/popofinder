-- 移除本檔種子資料(限 TPECH drview 來源,不誤刪其他 hospital_directory 資料)。
DELETE FROM medical_personnel
WHERE source = 'hospital_directory'
  AND source_url LIKE 'https://websrv01.tpech.gov.tw/drview/%';
