//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AppErrorReport {
  /// Returns a new [AppErrorReport] instance.
  AppErrorReport({
    required this.error,
    this.stack,
    this.version,
    this.build,
    this.screen,
    this.os,
  });

  /// Текст ошибки, от 1 до 2000 символов
  String error;

  /// Стек, до 20000 символов
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? stack;

  /// Версия приложения, до 32 символов
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? version;

  /// Номер сборки, от 0
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? build;

  /// Последний открытый экран, 1–32 символа `a-z` и `_` (specs/020-app-sessions.md, требование 10) 
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? screen;

  /// Система телефона, до 100 символов
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? os;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AppErrorReport &&
    other.error == error &&
    other.stack == stack &&
    other.version == version &&
    other.build == build &&
    other.screen == screen &&
    other.os == os;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (error.hashCode) +
    (stack == null ? 0 : stack!.hashCode) +
    (version == null ? 0 : version!.hashCode) +
    (build == null ? 0 : build!.hashCode) +
    (screen == null ? 0 : screen!.hashCode) +
    (os == null ? 0 : os!.hashCode);

  @override
  String toString() => 'AppErrorReport[error=$error, stack=$stack, version=$version, build=$build, screen=$screen, os=$os]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'error'] = this.error;
    if (this.stack != null) {
      json[r'stack'] = this.stack;
    } else {
      json[r'stack'] = null;
    }
    if (this.version != null) {
      json[r'version'] = this.version;
    } else {
      json[r'version'] = null;
    }
    if (this.build != null) {
      json[r'build'] = this.build;
    } else {
      json[r'build'] = null;
    }
    if (this.screen != null) {
      json[r'screen'] = this.screen;
    } else {
      json[r'screen'] = null;
    }
    if (this.os != null) {
      json[r'os'] = this.os;
    } else {
      json[r'os'] = null;
    }
    return json;
  }

  /// Returns a new [AppErrorReport] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AppErrorReport? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'error'), 'Required key "AppErrorReport[error]" is missing from JSON.');
        assert(json[r'error'] != null, 'Required key "AppErrorReport[error]" has a null value in JSON.');
        return true;
      }());

      return AppErrorReport(
        error: mapValueOfType<String>(json, r'error')!,
        stack: mapValueOfType<String>(json, r'stack'),
        version: mapValueOfType<String>(json, r'version'),
        build: mapValueOfType<int>(json, r'build'),
        screen: mapValueOfType<String>(json, r'screen'),
        os: mapValueOfType<String>(json, r'os'),
      );
    }
    return null;
  }

  static List<AppErrorReport> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AppErrorReport>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AppErrorReport.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AppErrorReport> mapFromJson(dynamic json) {
    final map = <String, AppErrorReport>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AppErrorReport.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AppErrorReport-objects as value to a dart map
  static Map<String, List<AppErrorReport>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AppErrorReport>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AppErrorReport.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'error',
  };
}

