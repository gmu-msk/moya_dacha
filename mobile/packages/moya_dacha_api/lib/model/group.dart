//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Group {
  /// Returns a new [Group] instance.
  Group({
    required this.id,
    required this.name,
    required this.description,
    required this.kind,
    required this.joinPolicy,
    this.owner,
    required this.members,
    required this.membership,
    this.place,
    this.distanceKm,
    required this.near,
    required this.createdAt,
  });

  /// Идентификатор группы (UUID)
  String id;

  String name;

  /// Может быть пустым
  String description;

  /// `interest` — по интересам, `place` — геогруппа
  GroupKindEnum kind;

  /// Всегда `open`; остальные значения — от первой версии
  GroupJoinPolicyEnum joinPolicy;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Author? owner;

  /// Сколько участников, с хозяином
  int members;

  /// Отношение смотрящего к группе; `requested` больше не бывает
  GroupMembershipEnum membership;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Place? place;

  /// Расстояние от пункта смотрящего до места геогруппы, округлённое как у поста; 0 — тот же пункт. Нет, если посчитать нельзя. 
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? distanceKm;

  /// Пункт смотрящего — место этой геогруппы
  bool near;

  DateTime createdAt;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Group &&
    other.id == id &&
    other.name == name &&
    other.description == description &&
    other.kind == kind &&
    other.joinPolicy == joinPolicy &&
    other.owner == owner &&
    other.members == members &&
    other.membership == membership &&
    other.place == place &&
    other.distanceKm == distanceKm &&
    other.near == near &&
    other.createdAt == createdAt;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (name.hashCode) +
    (description.hashCode) +
    (kind.hashCode) +
    (joinPolicy.hashCode) +
    (owner == null ? 0 : owner!.hashCode) +
    (members.hashCode) +
    (membership.hashCode) +
    (place == null ? 0 : place!.hashCode) +
    (distanceKm == null ? 0 : distanceKm!.hashCode) +
    (near.hashCode) +
    (createdAt.hashCode);

  @override
  String toString() => 'Group[id=$id, name=$name, description=$description, kind=$kind, joinPolicy=$joinPolicy, owner=$owner, members=$members, membership=$membership, place=$place, distanceKm=$distanceKm, near=$near, createdAt=$createdAt]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'name'] = this.name;
      json[r'description'] = this.description;
      json[r'kind'] = this.kind;
      json[r'join_policy'] = this.joinPolicy;
    if (this.owner != null) {
      json[r'owner'] = this.owner;
    } else {
      json[r'owner'] = null;
    }
      json[r'members'] = this.members;
      json[r'membership'] = this.membership;
    if (this.place != null) {
      json[r'place'] = this.place;
    } else {
      json[r'place'] = null;
    }
    if (this.distanceKm != null) {
      json[r'distance_km'] = this.distanceKm;
    } else {
      json[r'distance_km'] = null;
    }
      json[r'near'] = this.near;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
    return json;
  }

  /// Returns a new [Group] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Group? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Group[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Group[id]" has a null value in JSON.');
        assert(json.containsKey(r'name'), 'Required key "Group[name]" is missing from JSON.');
        assert(json[r'name'] != null, 'Required key "Group[name]" has a null value in JSON.');
        assert(json.containsKey(r'description'), 'Required key "Group[description]" is missing from JSON.');
        assert(json[r'description'] != null, 'Required key "Group[description]" has a null value in JSON.');
        assert(json.containsKey(r'kind'), 'Required key "Group[kind]" is missing from JSON.');
        assert(json[r'kind'] != null, 'Required key "Group[kind]" has a null value in JSON.');
        assert(json.containsKey(r'join_policy'), 'Required key "Group[join_policy]" is missing from JSON.');
        assert(json[r'join_policy'] != null, 'Required key "Group[join_policy]" has a null value in JSON.');
        assert(json.containsKey(r'members'), 'Required key "Group[members]" is missing from JSON.');
        assert(json[r'members'] != null, 'Required key "Group[members]" has a null value in JSON.');
        assert(json.containsKey(r'membership'), 'Required key "Group[membership]" is missing from JSON.');
        assert(json[r'membership'] != null, 'Required key "Group[membership]" has a null value in JSON.');
        assert(json.containsKey(r'near'), 'Required key "Group[near]" is missing from JSON.');
        assert(json[r'near'] != null, 'Required key "Group[near]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "Group[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "Group[created_at]" has a null value in JSON.');
        return true;
      }());

      return Group(
        id: mapValueOfType<String>(json, r'id')!,
        name: mapValueOfType<String>(json, r'name')!,
        description: mapValueOfType<String>(json, r'description')!,
        kind: GroupKindEnum.fromJson(json[r'kind'])!,
        joinPolicy: GroupJoinPolicyEnum.fromJson(json[r'join_policy'])!,
        owner: Author.fromJson(json[r'owner']),
        members: mapValueOfType<int>(json, r'members')!,
        membership: GroupMembershipEnum.fromJson(json[r'membership'])!,
        place: Place.fromJson(json[r'place']),
        distanceKm: mapValueOfType<int>(json, r'distance_km'),
        near: mapValueOfType<bool>(json, r'near')!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
      );
    }
    return null;
  }

  static List<Group> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Group>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Group.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Group> mapFromJson(dynamic json) {
    final map = <String, Group>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Group.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Group-objects as value to a dart map
  static Map<String, List<Group>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Group>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Group.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'name',
    'description',
    'kind',
    'join_policy',
    'members',
    'membership',
    'near',
    'created_at',
  };
}

/// `interest` — по интересам, `place` — геогруппа
enum GroupKindEnum {
  interest._(r'interest'),
  place._(r'place'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupKindEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupKindEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupKindEnum? fromJson(dynamic value) => GroupKindEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupKindEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupKindEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupKindEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupKindEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupKindEnum] to String,
/// and [decode] dynamic data back to [GroupKindEnum].
class GroupKindEnumTypeTransformer {
  factory GroupKindEnumTypeTransformer() => _instance ??= const GroupKindEnumTypeTransformer._();

  const GroupKindEnumTypeTransformer._();

  String encode(GroupKindEnum data) => data._value;

  /// Returns the instance of [GroupKindEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupKindEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupKindEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'interest': return GroupKindEnum.interest;
        case r'place': return GroupKindEnum.place;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupKindEnumTypeTransformer? _instance;
}


/// Всегда `open`; остальные значения — от первой версии
enum GroupJoinPolicyEnum {
  open._(r'open'),
  request._(r'request'),
  invite._(r'invite'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupJoinPolicyEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupJoinPolicyEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupJoinPolicyEnum? fromJson(dynamic value) => GroupJoinPolicyEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupJoinPolicyEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupJoinPolicyEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupJoinPolicyEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupJoinPolicyEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupJoinPolicyEnum] to String,
/// and [decode] dynamic data back to [GroupJoinPolicyEnum].
class GroupJoinPolicyEnumTypeTransformer {
  factory GroupJoinPolicyEnumTypeTransformer() => _instance ??= const GroupJoinPolicyEnumTypeTransformer._();

  const GroupJoinPolicyEnumTypeTransformer._();

  String encode(GroupJoinPolicyEnum data) => data._value;

  /// Returns the instance of [GroupJoinPolicyEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupJoinPolicyEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupJoinPolicyEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'open': return GroupJoinPolicyEnum.open;
        case r'request': return GroupJoinPolicyEnum.request;
        case r'invite': return GroupJoinPolicyEnum.invite;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupJoinPolicyEnumTypeTransformer? _instance;
}


/// Отношение смотрящего к группе; `requested` больше не бывает
enum GroupMembershipEnum {
  owner._(r'owner'),
  member._(r'member'),
  requested._(r'requested'),
  invited._(r'invited'),
  none._(r'none'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupMembershipEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupMembershipEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupMembershipEnum? fromJson(dynamic value) => GroupMembershipEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupMembershipEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupMembershipEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupMembershipEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupMembershipEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupMembershipEnum] to String,
/// and [decode] dynamic data back to [GroupMembershipEnum].
class GroupMembershipEnumTypeTransformer {
  factory GroupMembershipEnumTypeTransformer() => _instance ??= const GroupMembershipEnumTypeTransformer._();

  const GroupMembershipEnumTypeTransformer._();

  String encode(GroupMembershipEnum data) => data._value;

  /// Returns the instance of [GroupMembershipEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupMembershipEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupMembershipEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'owner': return GroupMembershipEnum.owner;
        case r'member': return GroupMembershipEnum.member;
        case r'requested': return GroupMembershipEnum.requested;
        case r'invited': return GroupMembershipEnum.invited;
        case r'none': return GroupMembershipEnum.none;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupMembershipEnumTypeTransformer? _instance;
}


