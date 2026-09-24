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
    required this.delivery,
    required this.resendAfter,
    required this.codeTtl,
  });

  /// Как человек получит код. `sent` — сервис отправил код на номер. `invite` — сервис в режиме приглашений, код у человека уже есть в приглашении, и сроки ниже равны нулю (specs/015-invites.md). 
  AuthCodeAcceptedDeliveryEnum delivery;

  /// Через сколько секунд можно запросить код снова
  int resendAfter;

  /// Сколько секунд живёт выданный код
  int codeTtl;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AuthCodeAccepted &&
    other.delivery == delivery &&
    other.resendAfter == resendAfter &&
    other.codeTtl == codeTtl;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (delivery.hashCode) +
    (resendAfter.hashCode) +
    (codeTtl.hashCode);

  @override
  String toString() => 'AuthCodeAccepted[delivery=$delivery, resendAfter=$resendAfter, codeTtl=$codeTtl]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'delivery'] = this.delivery;
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
        assert(json.containsKey(r'delivery'), 'Required key "AuthCodeAccepted[delivery]" is missing from JSON.');
        assert(json[r'delivery'] != null, 'Required key "AuthCodeAccepted[delivery]" has a null value in JSON.');
        assert(json.containsKey(r'resend_after'), 'Required key "AuthCodeAccepted[resend_after]" is missing from JSON.');
        assert(json[r'resend_after'] != null, 'Required key "AuthCodeAccepted[resend_after]" has a null value in JSON.');
        assert(json.containsKey(r'code_ttl'), 'Required key "AuthCodeAccepted[code_ttl]" is missing from JSON.');
        assert(json[r'code_ttl'] != null, 'Required key "AuthCodeAccepted[code_ttl]" has a null value in JSON.');
        return true;
      }());

      return AuthCodeAccepted(
        delivery: AuthCodeAcceptedDeliveryEnum.fromJson(json[r'delivery'])!,
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
    'delivery',
    'resend_after',
    'code_ttl',
  };
}

/// Как человек получит код. `sent` — сервис отправил код на номер. `invite` — сервис в режиме приглашений, код у человека уже есть в приглашении, и сроки ниже равны нулю (specs/015-invites.md). 
enum AuthCodeAcceptedDeliveryEnum {
  sent._(r'sent'),
  invite._(r'invite'),
  ;

  /// Instantiate a new enum with the provided value.
  const AuthCodeAcceptedDeliveryEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [AuthCodeAcceptedDeliveryEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static AuthCodeAcceptedDeliveryEnum? fromJson(dynamic value) => AuthCodeAcceptedDeliveryEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [AuthCodeAcceptedDeliveryEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<AuthCodeAcceptedDeliveryEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AuthCodeAcceptedDeliveryEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AuthCodeAcceptedDeliveryEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [AuthCodeAcceptedDeliveryEnum] to String,
/// and [decode] dynamic data back to [AuthCodeAcceptedDeliveryEnum].
class AuthCodeAcceptedDeliveryEnumTypeTransformer {
  factory AuthCodeAcceptedDeliveryEnumTypeTransformer() => _instance ??= const AuthCodeAcceptedDeliveryEnumTypeTransformer._();

  const AuthCodeAcceptedDeliveryEnumTypeTransformer._();

  String encode(AuthCodeAcceptedDeliveryEnum data) => data._value;

  /// Returns the instance of [AuthCodeAcceptedDeliveryEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  AuthCodeAcceptedDeliveryEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is AuthCodeAcceptedDeliveryEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'sent': return AuthCodeAcceptedDeliveryEnum.sent;
        case r'invite': return AuthCodeAcceptedDeliveryEnum.invite;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static AuthCodeAcceptedDeliveryEnumTypeTransformer? _instance;
}


