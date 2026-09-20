//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AuthCodeAccepted {
  /// Returns a new [AuthCodeAccepted] instance.
  AuthCodeAccepted({
    required this.resendAfter,
    required this.codeTtl,
  });

  /// Через сколько секунд можно запросить код снова
  int resendAfter;

  /// Сколько секунд живёт выданный код
  int codeTtl;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AuthCodeAccepted &&
    other.resendAfter == resendAfter &&
    other.codeTtl == codeTtl;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (resendAfter.hashCode) +
    (codeTtl.hashCode);

  @override
  String toString() => 'AuthCodeAccepted[resendAfter=$resendAfter, codeTtl=$codeTtl]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'resend_after'] = this.resendAfter;
      json[r'code_ttl'] = this.codeTtl;
    return json;
  }

  /// Returns a new [AuthCodeAccepted] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AuthCodeAccepted? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'resend_after'), 'Required key "AuthCodeAccepted[resend_after]" is missing from JSON.');
        assert(json[r'resend_after'] != null, 'Required key "AuthCodeAccepted[resend_after]" has a null value in JSON.');
        assert(json.containsKey(r'code_ttl'), 'Required key "AuthCodeAccepted[code_ttl]" is missing from JSON.');
        assert(json[r'code_ttl'] != null, 'Required key "AuthCodeAccepted[code_ttl]" has a null value in JSON.');
        return true;
      }());

      return AuthCodeAccepted(
        resendAfter: mapValueOfType<int>(json, r'resend_after')!,
        codeTtl: mapValueOfType<int>(json, r'code_ttl')!,
      );
    }
    return null;
  }

  static List<AuthCodeAccepted> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AuthCodeAccepted>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AuthCodeAccepted.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AuthCodeAccepted> mapFromJson(dynamic json) {
    final map = <String, AuthCodeAccepted>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AuthCodeAccepted.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AuthCodeAccepted-objects as value to a dart map
  static Map<String, List<AuthCodeAccepted>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AuthCodeAccepted>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AuthCodeAccepted.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'resend_after',
    'code_ttl',
  };
}

