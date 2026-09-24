//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Notification {
  /// Returns a new [Notification] instance.
  Notification({
    required this.id,
    required this.kind,
    required this.createdAt,
    required this.actor,
    this.others,
    this.post,
    this.comment,
    required this.unread,
  });

  /// Идентификатор строки (UUID)
  String id;

  /// `follow` — подписался на вас, `follow_accepted` — принял вашу заявку, `like` — отметил ваш пост, `comment` — ответил на ваш пост. 
  NotificationKindEnum kind;

  /// Когда это случилось
  DateTime createdAt;

  FollowUser actor;

  /// Только у `like` — сколько ещё людей отметили пост
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? others;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  NotificationPost? post;

  /// Только у `comment` — начало комментария, до 100 символов
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? comment;

  /// Новее ли строка последнего открытия раздела
  bool unread;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Notification &&
    other.id == id &&
    other.kind == kind &&
    other.createdAt == createdAt &&
    other.actor == actor &&
    other.others == others &&
    other.post == post &&
    other.comment == comment &&
    other.unread == unread;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (kind.hashCode) +
    (createdAt.hashCode) +
    (actor.hashCode) +
    (others == null ? 0 : others!.hashCode) +
    (post == null ? 0 : post!.hashCode) +
    (comment == null ? 0 : comment!.hashCode) +
    (unread.hashCode);

  @override
  String toString() => 'Notification[id=$id, kind=$kind, createdAt=$createdAt, actor=$actor, others=$others, post=$post, comment=$comment, unread=$unread]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'kind'] = this.kind;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'actor'] = this.actor;
    if (this.others != null) {
      json[r'others'] = this.others;
    } else {
      json[r'others'] = null;
    }
    if (this.post != null) {
      json[r'post'] = this.post;
    } else {
      json[r'post'] = null;
    }
    if (this.comment != null) {
      json[r'comment'] = this.comment;
    } else {
      json[r'comment'] = null;
    }
      json[r'unread'] = this.unread;
    return json;
  }

  /// Returns a new [Notification] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Notification? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Notification[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Notification[id]" has a null value in JSON.');
        assert(json.containsKey(r'kind'), 'Required key "Notification[kind]" is missing from JSON.');
        assert(json[r'kind'] != null, 'Required key "Notification[kind]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "Notification[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "Notification[created_at]" has a null value in JSON.');
        assert(json.containsKey(r'actor'), 'Required key "Notification[actor]" is missing from JSON.');
        assert(json[r'actor'] != null, 'Required key "Notification[actor]" has a null value in JSON.');
        assert(json.containsKey(r'unread'), 'Required key "Notification[unread]" is missing from JSON.');
        assert(json[r'unread'] != null, 'Required key "Notification[unread]" has a null value in JSON.');
        return true;
      }());

      return Notification(
        id: mapValueOfType<String>(json, r'id')!,
        kind: NotificationKindEnum.fromJson(json[r'kind'])!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
        actor: FollowUser.fromJson(json[r'actor'])!,
        others: mapValueOfType<int>(json, r'others'),
        post: NotificationPost.fromJson(json[r'post']),
        comment: mapValueOfType<String>(json, r'comment'),
        unread: mapValueOfType<bool>(json, r'unread')!,
      );
    }
    return null;
  }

  static List<Notification> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Notification>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Notification.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Notification> mapFromJson(dynamic json) {
    final map = <String, Notification>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Notification.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Notification-objects as value to a dart map
  static Map<String, List<Notification>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Notification>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Notification.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'kind',
    'created_at',
    'actor',
    'unread',
  };
}

/// `follow` — подписался на вас, `follow_accepted` — принял вашу заявку, `like` — отметил ваш пост, `comment` — ответил на ваш пост. 
enum NotificationKindEnum {
  follow._(r'follow'),
  followAccepted._(r'follow_accepted'),
  like._(r'like'),
  comment._(r'comment'),
  ;

  /// Instantiate a new enum with the provided value.
  const NotificationKindEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [NotificationKindEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static NotificationKindEnum? fromJson(dynamic value) => NotificationKindEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [NotificationKindEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<NotificationKindEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <NotificationKindEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = NotificationKindEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [NotificationKindEnum] to String,
/// and [decode] dynamic data back to [NotificationKindEnum].
class NotificationKindEnumTypeTransformer {
  factory NotificationKindEnumTypeTransformer() => _instance ??= const NotificationKindEnumTypeTransformer._();

  const NotificationKindEnumTypeTransformer._();

  String encode(NotificationKindEnum data) => data._value;

  /// Returns the instance of [NotificationKindEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  NotificationKindEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is NotificationKindEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'follow': return NotificationKindEnum.follow;
        case r'follow_accepted': return NotificationKindEnum.followAccepted;
        case r'like': return NotificationKindEnum.like;
        case r'comment': return NotificationKindEnum.comment;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static NotificationKindEnumTypeTransformer? _instance;
}


