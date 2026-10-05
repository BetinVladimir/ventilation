# Состояние npm audit — 4 октября 2026

В application/package.json добавлен узкий override xcode → uuid 11.1.1. Версия поддерживает CommonJS и используемый xcode метод v4. Lockfile обновлён. Устранены все 6 moderate замечаний, связанных с uuid <11.1.1.

Остаются 21 high замечание из двух корневых уязвимостей; остальные записи — их зависимые пакеты:

- braces 3.0.3: GHSA-vfj7-8cjw-p6xm, переполнение стека на глубоко вложенных шаблонах. https://github.com/advisories/GHSA-vfj7-8cjw-p6xm
- node-forge 1.4.0: GHSA-86w9-cpqp-85rv, некорректная проверка вложенных ASN.1 элементов при проверке RSA PKCS#1 v1.5. https://github.com/advisories/GHSA-86w9-cpqp-85rv

Обе версии — последние опубликованные в npm; в advisory Patched versions: None. Последняя Expo CLI также зависит от node-forge; Metro продолжает использовать micromatch → braces. Обновление Expo не устраняет обе причины.

Не применялись подмена имён пакетов, фиктивные версии, исключения audit и npm audit fix --force: они не подтверждают устранение уязвимостей.

Полное устранение требует проверенных патчей библиотек или замены зависимых инструментов. При появлении исправленных релизов обновить зависимости и повторить npm ci, npm audit, npm run typecheck, npx expo install --check и npx expo export --platform android.
