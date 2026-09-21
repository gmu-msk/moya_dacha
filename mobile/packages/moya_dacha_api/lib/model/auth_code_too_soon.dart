//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AuthCodeTooSoon {
  /// Returns a new [AuthCodeTooSoon] instance.
  AuthCodeTooSoon({
    required this.code,
    required this.message,
    required this.retryAfter,
  });

  /// Машиночитаемый код ошибки
  String code;

  /// Человекочитаемое описание, пригодное для показа пользователю
  String message;

  /// Через сколько секунд можно запросить код снова
  ///
  /// Minimum value: 1
  int retryAfter;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AuthCodeTooSoon &&
    other.code == code &&
    other.message == message &&
    other.retryAfter == retryAfter;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (code.hashCode) +
    (message.hashCode) +
    (retryAfter.hashCode);

  @override
  String toString() => 'AuthCodeTooSoon[code=$code, message=$message, retryAfter=$retryAfter]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'code'] = this.code;
      json[r'message'] = this.message;
      json[r'retry_after'] = this.retryAfter;
    return json;
  }

  /// Returns a new [AuthCodeTooSoon] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AuthCodeTooSoon? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'code'), 'Required key "AuthCodeTooSoon[code]" is missing from JSON.');
        assert(json[r'code'] != null, 'Required key "AuthCodeTooSoon[code]" has a null value in JSON.');
        assert(json.containsKey(r'message'), 'Required key "AuthCodeTooSoon[message]" is missing from JSON.');
        assert(json[r'message'] != null, 'Required key "AuthCodeTooSoon[message]" has a null value in JSON.');
        assert(json.containsKey(r'retry_after'), 'Required key "AuthCodeTooSoon[retry_after]" is missing from JSON.');
        assert(json[r'retry_after'] != null, 'Required key "AuthCodeTooSoon[retry_after]" has a null value in JSON.');
        return true;
      }());

      return AuthCodeTooSoon(
        code: mapValueOfType<String>(json, r'code')!,
        message: mapValueOfType<String>(json, r'message')!,
        retryAfter: mapValueOfType<int>(json, r'retry_after')!,
      );
    }
    return null;
  }

  static List<AuthCodeTooSoon> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AuthCodeTooSoon>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AuthCodeTooSoon.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AuthCodeTooSoon> mapFromJson(dynamic json) {
    final map = <String, AuthCodeTooSoon>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AuthCodeTooSoon.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AuthCodeTooSoon-objects as value to a dart map
  static Map<String, List<AuthCodeTooSoon>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AuthCodeTooSoon>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AuthCodeTooSoon.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'code',
    'message',
    'retry_after',
  };
}

