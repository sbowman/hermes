# Changelog

## [Unreleased]

### Added

- Fixed bug where `Tx.Close()` returned an error if the underlying database transaction had already closed.


## [1.2.4] - 2020-01-11

### Added

- Support for transaction savepoints (https://www.postgresql.org/docs/12/sql-savepoint.html)


## [1.2.3] - 2015-12-03

### Added
- Connection confirmation: Hermes may be configured to retry the database connection before panicking. 


[Unreleased]: https://github.com/sbowman/hermes/compare/v1.2.4...HEAD
[1.2.4]: https://github.com/sbowman/hermes/compare/v1.2.3...v1.2.4
[1.2.3]: https://github.com/sbowman/hermes/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/sbowman/hermes/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/sbowman/hermes/compare/v1.1.0...v1.2.2
[1.1.0]: https://github.com/sbowman/hermes/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/sbowman/hermes/releases/tag/v1.0.0
