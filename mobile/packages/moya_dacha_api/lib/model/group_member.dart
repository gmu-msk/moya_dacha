//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class GroupMember {
  /// Returns a new [GroupMember] instance.
  GroupMember({
    required this.user,
    required this.role,
    required this.state,
    required this.createdAt,
  });

  Author user;

  GroupMemberRoleEnum role;

  GroupMemberStateEnum state;

  /// Когда вступил или приглашён
  DateTime createdAt;

  @override
  bool operator ==(Object other) => identical(this, other) || other is GroupMember &&
    other.user == user &&
    other.role == role &&
    other.state == state &&
    other.createdAt == createdAt;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (user.hashCode) +
    (role.hashCode) +
    (state.hashCode) +
    (createdAt.hashCode);

  @override
  String toString() => 'GroupMember[user=$user, role=$role, state=$state, createdAt=$createdAt]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'user'] = this.user;
      json[r'role'] = this.role;
      json[r'state'] = this.state;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
    return json;
  }

  /// Returns a new [GroupMember] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static GroupMember? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'user'), 'Required key "GroupMember[user]" is missing from JSON.');
        assert(json[r'user'] != null, 'Required key "GroupMember[user]" has a null value in JSON.');
        assert(json.containsKey(r'role'), 'Required key "GroupMember[role]" is missing from JSON.');
        assert(json[r'role'] != null, 'Required key "GroupMember[role]" has a null value in JSON.');
        assert(json.containsKey(r'state'), 'Required key "GroupMember[state]" is missing from JSON.');
        assert(json[r'state'] != null, 'Required key "GroupMember[state]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "GroupMember[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "GroupMember[created_at]" has a null value in JSON.');
        return true;
      }());

      return GroupMember(
        user: Author.fromJson(json[r'user'])!,
        role: GroupMemberRoleEnum.fromJson(json[r'role'])!,
        state: GroupMemberStateEnum.fromJson(json[r'state'])!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
      );
    }
    return null;
  }

  static List<GroupMember> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupMember>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupMember.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, GroupMember> mapFromJson(dynamic json) {
    final map = <String, GroupMember>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = GroupMember.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of GroupMember-objects as value to a dart map
  static Map<String, List<GroupMember>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<GroupMember>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = GroupMember.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'user',
    'role',
    'state',
    'created_at',
  };
}


enum GroupMemberRoleEnum {
  owner._(r'owner'),
  member._(r'member'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupMemberRoleEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupMemberRoleEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupMemberRoleEnum? fromJson(dynamic value) => GroupMemberRoleEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupMemberRoleEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupMemberRoleEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupMemberRoleEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupMemberRoleEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupMemberRoleEnum] to String,
/// and [decode] dynamic data back to [GroupMemberRoleEnum].
class GroupMemberRoleEnumTypeTransformer {
  factory GroupMemberRoleEnumTypeTransformer() => _instance ??= const GroupMemberRoleEnumTypeTransformer._();

  const GroupMemberRoleEnumTypeTransformer._();

  String encode(GroupMemberRoleEnum data) => data._value;

  /// Returns the instance of [GroupMemberRoleEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupMemberRoleEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupMemberRoleEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'owner': return GroupMemberRoleEnum.owner;
        case r'member': return GroupMemberRoleEnum.member;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupMemberRoleEnumTypeTransformer? _instance;
}



enum GroupMemberStateEnum {
  member._(r'member'),
  requested._(r'requested'),
  invited._(r'invited'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupMemberStateEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupMemberStateEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupMemberStateEnum? fromJson(dynamic value) => GroupMemberStateEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupMemberStateEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupMemberStateEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupMemberStateEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupMemberStateEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupMemberStateEnum] to String,
/// and [decode] dynamic data back to [GroupMemberStateEnum].
class GroupMemberStateEnumTypeTransformer {
  factory GroupMemberStateEnumTypeTransformer() => _instance ??= const GroupMemberStateEnumTypeTransformer._();

  const GroupMemberStateEnumTypeTransformer._();

  String encode(GroupMemberStateEnum data) => data._value;

  /// Returns the instance of [GroupMemberStateEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupMemberStateEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupMemberStateEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'member': return GroupMemberStateEnum.member;
        case r'requested': return GroupMemberStateEnum.requested;
        case r'invited': return GroupMemberStateEnum.invited;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupMemberStateEnumTypeTransformer? _instance;
}


