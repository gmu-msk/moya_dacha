//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class CurrentUser {
  /// Returns a new [CurrentUser] instance.
  CurrentUser({
    required this.id,
    required this.phone,
    required this.createdAt,
  });

  /// Идентификатор пользователя (UUID)
  String id;

  /// Нормализованный номер телефона
  String phone;

  /// Когда пользователь зарегистрировался
  DateTime createdAt;

  @override
  bool operator ==(Object other) => identical(this, other) || other is CurrentUser &&
    other.id == id &&
    other.phone == phone &&
    other.createdAt == createdAt;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (phone.hashCode) +
    (createdAt.hashCode);

  @override
  String toString() => 'CurrentUser[id=$id, phone=$phone, createdAt=$createdAt]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'phone'] = this.phone;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
    return json;
  }

  /// Returns a new [CurrentUser] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static CurrentUser? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "CurrentUser[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "CurrentUser[id]" has a null value in JSON.');
        assert(json.containsKey(r'phone'), 'Required key "CurrentUser[phone]" is missing from JSON.');
        assert(json[r'phone'] != null, 'Required key "CurrentUser[phone]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "CurrentUser[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "CurrentUser[created_at]" has a null value in JSON.');
        return true;
      }());

      return CurrentUser(
        id: mapValueOfType<String>(json, r'id')!,
        phone: mapValueOfType<String>(json, r'phone')!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
      );
    }
    return null;
  }

  static List<CurrentUser> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <CurrentUser>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = CurrentUser.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, CurrentUser> mapFromJson(dynamic json) {
    final map = <String, CurrentUser>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = CurrentUser.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of CurrentUser-objects as value to a dart map
  static Map<String, List<CurrentUser>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<CurrentUser>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = CurrentUser.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'phone',
    'created_at',
  };
}

