//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AppSessionEnd {
  /// Returns a new [AppSessionEnd] instance.
  AppSessionEnd({
    this.screens = const {},
  });

  /// Сколько раз открывался каждый экран за всю сессию — счётчик накопительный (specs/020-app-sessions.md, требования 8–10) 
  Map<String, int> screens;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AppSessionEnd &&
    _deepEquality.equals(other.screens, screens);

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (screens.hashCode);

  @override
  String toString() => 'AppSessionEnd[screens=$screens]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'screens'] = this.screens;
    return json;
  }

  /// Returns a new [AppSessionEnd] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AppSessionEnd? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        return true;
      }());

      return AppSessionEnd(
        screens: mapCastOfType<String, int>(json, r'screens') ?? const {},
      );
    }
    return null;
  }

  static List<AppSessionEnd> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AppSessionEnd>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AppSessionEnd.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AppSessionEnd> mapFromJson(dynamic json) {
    final map = <String, AppSessionEnd>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AppSessionEnd.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AppSessionEnd-objects as value to a dart map
  static Map<String, List<AppSessionEnd>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AppSessionEnd>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AppSessionEnd.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
  };
}

