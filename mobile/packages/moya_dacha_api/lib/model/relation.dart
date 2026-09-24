//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Relation {
  /// Returns a new [Relation] instance.
  Relation({
    required this.following,
    required this.followedBy,
  });

  /// Смотрящий → пользователь: `none` — не подписан, `requested` — заявка ждёт ответа, `yes` — подписан. 
  RelationFollowingEnum following;

  /// Подписан ли пользователь на смотрящего (заявка не в счёт)
  bool followedBy;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Relation &&
    other.following == following &&
    other.followedBy == followedBy;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (following.hashCode) +
    (followedBy.hashCode);

  @override
  String toString() => 'Relation[following=$following, followedBy=$followedBy]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'following'] = this.following;
      json[r'followed_by'] = this.followedBy;
    return json;
  }

  /// Returns a new [Relation] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Relation? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'following'), 'Required key "Relation[following]" is missing from JSON.');
        assert(json[r'following'] != null, 'Required key "Relation[following]" has a null value in JSON.');
        assert(json.containsKey(r'followed_by'), 'Required key "Relation[followed_by]" is missing from JSON.');
        assert(json[r'followed_by'] != null, 'Required key "Relation[followed_by]" has a null value in JSON.');
        return true;
      }());

      return Relation(
        following: RelationFollowingEnum.fromJson(json[r'following'])!,
        followedBy: mapValueOfType<bool>(json, r'followed_by')!,
      );
    }
    return null;
  }

  static List<Relation> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Relation>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Relation.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Relation> mapFromJson(dynamic json) {
    final map = <String, Relation>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Relation.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Relation-objects as value to a dart map
  static Map<String, List<Relation>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Relation>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Relation.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'following',
    'followed_by',
  };
}

/// Смотрящий → пользователь: `none` — не подписан, `requested` — заявка ждёт ответа, `yes` — подписан. 
enum RelationFollowingEnum {
  none._(r'none'),
  requested._(r'requested'),
  yes._(r'yes'),
  ;

  /// Instantiate a new enum with the provided value.
  const RelationFollowingEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [RelationFollowingEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static RelationFollowingEnum? fromJson(dynamic value) => RelationFollowingEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [RelationFollowingEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<RelationFollowingEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <RelationFollowingEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = RelationFollowingEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [RelationFollowingEnum] to String,
/// and [decode] dynamic data back to [RelationFollowingEnum].
class RelationFollowingEnumTypeTransformer {
  factory RelationFollowingEnumTypeTransformer() => _instance ??= const RelationFollowingEnumTypeTransformer._();

  const RelationFollowingEnumTypeTransformer._();

  String encode(RelationFollowingEnum data) => data._value;

  /// Returns the instance of [RelationFollowingEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  RelationFollowingEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is RelationFollowingEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'none': return RelationFollowingEnum.none;
        case r'requested': return RelationFollowingEnum.requested;
        case r'yes': return RelationFollowingEnum.yes;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static RelationFollowingEnumTypeTransformer? _instance;
}


