#!/bin/bash
#
# Checks mounted partitions' usage space and alerts thru AWS SNS if it's greater than MAX_USAGE percents
#

#-- Set the SNS topic ARN here
TOPIC_ARN=$1
MAX_USAGE=90

[ "$TOPIC_ARN" = "" ] && { echo "SNS topic ARN is not set, exiting.."; exit; }

EC2_AVAIL_ZONE=`curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone`
EC2_REGION="`echo \"$EC2_AVAIL_ZONE\" | sed 's/[a-z]$//'`"

[ "$EC2_REGION" = "" ] && { echo "EC2 region is unknown, exiting.."; exit; }

for j in $(df -h | grep -E "/dev/x|/dev/n" \
    | awk '{print $1,$5}' \
    | sort -u \
    | cut -f2 -d' ' \
    | sed -e 's/%//g' \
    | sort -u); do
        [ $j -gt $MAX_USAGE ] && echo -e "One of the volumes have reached ${MAX_USAGE}% of usage:\n\n$(df -h)" \
    	    | aws --region $EC2_REGION sns publish --topic-arn "$TOPIC_ARN" --subject "ALARM: host $(hostname) is lack of disk space" --message file:///dev/stdin &>/dev/null
done
